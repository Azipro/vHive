// MIT License
//
// Copyright (c) 2023 Georgiy Lebedev, Amory Hoste and vHive team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package snapshotting

import (
	"fmt"
	"github.com/pkg/errors"
	"os"
	"sync"
)

// SnapshotManager manages snapshots stored on the node. Each snapshot can only be used by a single VM at
// a time and thus is always in one of three states: creating, active or idle.
type SnapshotManager struct {
	sync.Mutex
	// Snapshots currently in use by a function (identified by the id of the VM using the snapshot)
	activeSnapshots map[string]*Snapshot
	// Snapshots currently being created (identified by the id of the VM the snapshot is being created for)
	creatingSnapshots map[string]*Snapshot
	// Offloaded snapshots available for reuse by new VMs (identified by the image name of the snapshot)
	idleSnapshots map[string][]*Snapshot
	baseFolder    string
}

// Snapshot identified by VM id

func NewSnapshotManager(baseFolder string) *SnapshotManager {
	manager := new(SnapshotManager)
	manager.activeSnapshots = make(map[string]*Snapshot)
	manager.creatingSnapshots = make(map[string]*Snapshot)
	manager.idleSnapshots = make(map[string][]*Snapshot)
	manager.baseFolder = baseFolder

	// Clean & init basefolder
	_ = os.RemoveAll(manager.baseFolder)
	_ = os.MkdirAll(manager.baseFolder, os.ModePerm)

	return manager
}

// AcquireSnapshot returns an idle snapshot if one is available for the given image
func (mgr *SnapshotManager) AcquireSnapshot(image string) (*Snapshot, error) {
	mgr.Lock()
	defer mgr.Unlock()

	// Check if idle snapshot is available for the given image
	idles, ok := mgr.idleSnapshots[image]
	if !ok {
		mgr.idleSnapshots[image] = []*Snapshot{}
		return nil, errors.New(fmt.Sprintf("There is no snapshot available for image %s", image))
	}

	// Return snapshot for supplied image
	if len(idles) != 0 {
		snp := idles[0]
		mgr.idleSnapshots[image] = idles[1:]
		mgr.activeSnapshots[snp.GetId()] = snp
		return snp, nil
	}
	return nil, errors.New(fmt.Sprintf("There is no snapshot available fo rimage %s", image))
}

// ReleaseSnapshot releases the snapshot in use by the given VM for offloading so that it can get used to handle a new
// VM creation.
func (mgr *SnapshotManager) ReleaseSnapshot(vmID string) error {
	mgr.Lock()
	defer mgr.Unlock()

	snap, present := mgr.activeSnapshots[vmID]
	if !present {
		return errors.New(fmt.Sprintf("Get: Snapshot for container %s does not exist", vmID))
	}

	// Move snapshot from active to idle state
	delete(mgr.activeSnapshots, vmID)
	mgr.idleSnapshots[snap.Image] = append(mgr.idleSnapshots[snap.Image], snap)

	return nil
}

// InitSnapshot initializes a snapshot by initializing a new snapshot and moving it to the creating state. CommitSnapshot
// must be run to finalize the snapshot creation and make the snapshot available for use
func (mgr *SnapshotManager) InitSnapshot(vmID, image string) (*Snapshot, error) {
	mgr.Lock()

	if _, present := mgr.creatingSnapshots[vmID]; present {
		mgr.Unlock()
		return nil, errors.New(fmt.Sprintf("Add: Snapshot for vm %s already exists", vmID))
	}

	// Create snapshot object and move into creating state
	snap := NewSnapshot(vmID, mgr.baseFolder, image)
	mgr.creatingSnapshots[snap.GetId()] = snap
	mgr.Unlock()

	// Create directory to store snapshot data
	err := snap.CreateSnapDir()
	if err != nil {
		return nil, errors.Wrapf(err, "creating snapDir for snapshots %s", vmID)
	}

	return snap, nil
}

// CommitSnapshot finalizes the snapshot creation and makes it available for use by moving it into the idle state.
func (mgr *SnapshotManager) CommitSnapshot(vmID string) error {
	mgr.Lock()
	defer mgr.Unlock()

	// Move snapshot from creating to idle state
	snap, ok := mgr.creatingSnapshots[vmID]
	if !ok {
		return errors.New(fmt.Sprintf("There has no snapshot been created with vmID %s", vmID))
	}
	delete(mgr.creatingSnapshots, vmID)

	_, ok = mgr.idleSnapshots[snap.Image]
	if !ok {
		mgr.idleSnapshots[snap.Image] = []*Snapshot{}
	}

	mgr.idleSnapshots[snap.Image] = append(mgr.idleSnapshots[snap.Image], snap)

	return nil
}
