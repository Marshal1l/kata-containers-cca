// Copyright (c) 2016 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package virtcontainers

import (
	"context"
	"fmt"
	"runtime"
	"time"

	deviceApi "github.com/kata-containers/kata-containers/src/runtime/pkg/device/api"
	deviceConfig "github.com/kata-containers/kata-containers/src/runtime/pkg/device/config"
	"github.com/kata-containers/kata-containers/src/runtime/pkg/katautils/katatrace"
	resCtrl "github.com/kata-containers/kata-containers/src/runtime/pkg/resourcecontrol"
	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/pkg/compatoci"
	vcTypes "github.com/kata-containers/kata-containers/src/runtime/virtcontainers/types"
	"github.com/sirupsen/logrus"
)

// apiTracingTags defines tags for the trace span
var apiTracingTags = map[string]string{
	"source":    "runtime",
	"package":   "virtcontainers",
	"subsystem": "api",
}

func init() {
	runtime.LockOSThread()
}

var virtLog = logrus.WithField("source", "virtcontainers")

// SetLogger sets the logger for virtcontainers package.
func SetLogger(ctx context.Context, logger *logrus.Entry) {
	fields := virtLog.Data
	virtLog = logger.WithFields(fields)
	SetHypervisorLogger(virtLog) // TODO: this will move to hypervisors pkg
	deviceApi.SetLogger(virtLog)
	compatoci.SetLogger(virtLog)
	deviceConfig.SetLogger(virtLog)
	resCtrl.SetLogger(virtLog)
}

// CreateSandbox is the virtcontainers sandbox creation entry point.
// CreateSandbox creates a sandbox and its containers. It does not start them.
func CreateSandbox(ctx context.Context, sandboxConfig SandboxConfig, factory Factory, prestartHookFunc func(context.Context) error) (VCSandbox, error) {
	// f, err := os.OpenFile("~/kata-runtime.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	// if err == nil {
	// 	log.SetOutput(f)
	// } else {
	// 	log.Fatal("Failed to open log file")
	// }
	// t1 := time.Now()
	span, ctx := katatrace.Trace(ctx, virtLog, "CreateSandbox", apiTracingTags)
	defer span.End()

	s, err := createSandboxFromConfig(ctx, sandboxConfig, factory, prestartHookFunc)
	// t2 := time.Now()
	// duration := t2.Sub(t1)
	// durationMs := duration.Milliseconds()
	// log.Printf("createSandboxFromConfig use time %d ms", durationMs)
	return s, err
}

func createSandboxFromConfig(ctx context.Context, sandboxConfig SandboxConfig, factory Factory, prestartHookFunc func(context.Context) error) (_ *Sandbox, err error) {
	span, ctx := katatrace.Trace(ctx, virtLog, "createSandboxFromConfig", apiTracingTags)
	defer span.End()
	start := time.Now()
	// Create the sandbox.
	s, err := createSandbox(ctx, sandboxConfig, factory)
	if err != nil {
		return nil, err
	}
	virtLog.Info(fmt.Sprintf("[MZH] createSandbox TIME: %v", time.Since(start)))
	// Cleanup sandbox resources in case of any failure
	defer func() {
		if err != nil {
			s.Delete(ctx)
		}
	}()

	// network rollback
	defer func() {
		if err != nil {
			virtLog.Info("Removing network after failure in createSandbox")
			s.removeNetwork(ctx)
		}
	}()
	start = time.Now()
	// Create the sandbox network
	if err = s.createNetwork(ctx); err != nil {
		return nil, err
	}
	virtLog.Info(fmt.Sprintf("[MZH] createNetwork TIME: %v", time.Since(start)))
	start = time.Now()
	// Set the sandbox host cgroups.
	if err := s.setupResourceController(); err != nil {
		return nil, err
	}
	virtLog.Info(fmt.Sprintf("[MZH] setupResourceController TIME: %v", time.Since(start)))
	// Start the VM
	start = time.Now()
	if err = s.startVM(ctx, prestartHookFunc); err != nil {
		return nil, err
	}
	virtLog.Info(fmt.Sprintf("[MZH] startVM TIME: %v", time.Since(start)))
	// rollback to stop VM if error occurs
	defer func() {
		if err != nil {
			s.stopVM(ctx)
		}
	}()
	start = time.Now()
	s.postCreatedNetwork(ctx)
	virtLog.Info(fmt.Sprintf("[MZH] postCreatedNetwork TIME: %v", time.Since(start)))
	start = time.Now()
	if err = s.getAndStoreGuestDetails(ctx); err != nil {
		return nil, err
	}
	virtLog.Info(fmt.Sprintf("[MZH] getAndStoreGuestDetails TIME: %v", time.Since(start)))
	start = time.Now()
	// Create Containers
	if err = s.createContainers(ctx); err != nil {
		return nil, err
	}
	virtLog.Info(fmt.Sprintf("[MZH] createContainers TIME: %v", time.Since(start)))
	return s, nil
}

// CleanupContainer is used by shimv2 to stop and delete a container exclusively, once there is no container
// in the sandbox left, do stop the sandbox and delete it. Those serial operations will be done exclusively by
// locking the sandbox.
func CleanupContainer(ctx context.Context, sandboxID, containerID string, force bool) error {
	span, ctx := katatrace.Trace(ctx, virtLog, "CleanupContainer", apiTracingTags)
	defer span.End()

	if sandboxID == "" {
		return vcTypes.ErrNeedSandboxID
	}

	if containerID == "" {
		return vcTypes.ErrNeedContainerID
	}

	unlock, err := rwLockSandbox(sandboxID)
	if err != nil {
		return err
	}
	defer unlock()

	s, err := fetchSandbox(ctx, sandboxID)
	if err != nil {
		return err
	}
	defer s.Release(ctx)

	_, err = s.StopContainer(ctx, containerID, force)
	if err != nil && !force {
		return err
	}

	_, err = s.DeleteContainer(ctx, containerID)
	if err != nil && !force {
		return err
	}

	if len(s.GetAllContainers()) > 0 {
		return nil
	}

	if err = s.Stop(ctx, force); err != nil && !force {
		return err
	}

	if err = s.Delete(ctx); err != nil {
		return err
	}

	return nil
}
