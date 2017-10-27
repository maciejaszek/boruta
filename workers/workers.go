/*
 *  Copyright (c) 2017-2018 Samsung Electronics Co., Ltd All Rights Reserved
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License
 */

// Package workers is responsible for worker list management.
package workers

import (
	"crypto/rsa"
	"math"
	"net"
	"sync"

	. "git.tizen.org/tools/boruta"
	"git.tizen.org/tools/boruta/dryad/conf"
	"git.tizen.org/tools/boruta/rpc/dryad"
)

// UUID denotes a key in Capabilities where WorkerUUID is stored.
const UUID string = "UUID"

// mapWorker is used by WorkerList to store all
// (public and private) structures representing Worker.
type mapWorker struct {
	WorkerInfo
	ip  net.IP
	key *rsa.PrivateKey
}

// WorkerList implements Superviser and Workers interfaces.
// It manages a list of Workers.
// It implements also WorkersManager from matcher package making it usable
// as interface for acquiring workers by Matcher.
// The implemnetation requires changeListener, which is notified after Worker's
// state changes.
// The dryad.ClientManager allows managing Dryads' clients for key generation.
// One can be created using newDryadClient function.
type WorkerList struct {
	Superviser
	Workers
	workers        map[WorkerUUID]*mapWorker
	mutex          *sync.RWMutex
	changeListener WorkerChange
	newDryadClient func() dryad.ClientManager
}

// newDryadClient provides default implementation of dryad.ClientManager interface.
// It uses dryad package implementation of DryadClient.
// The function is set as WorkerList.newDryadClient. Field can be replaced
// by another function providing dryad.ClientManager for tests purposes.
func newDryadClient() dryad.ClientManager {
	return new(dryad.DryadClient)
}

// NewWorkerList returns a new WorkerList with all fields set.
func NewWorkerList() *WorkerList {
	return &WorkerList{
		workers:        make(map[WorkerUUID]*mapWorker),
		mutex:          new(sync.RWMutex),
		newDryadClient: newDryadClient,
	}
}

// Register is an implementation of Register from Superviser interface.
// UUID, which identifies Worker, must be present in caps.
func (wl *WorkerList) Register(caps Capabilities) error {
	capsUUID, present := caps[UUID]
	if !present {
		return ErrMissingUUID
	}
	uuid := WorkerUUID(capsUUID)
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, registered := wl.workers[uuid]
	if registered {
		// Subsequent Register calls update the caps.
		worker.Caps = caps
	} else {
		wl.workers[uuid] = &mapWorker{
			WorkerInfo: WorkerInfo{
				WorkerUUID: uuid,
				State:      MAINTENANCE,
				Caps:       caps,
			}}
	}
	return nil
}

// SetFail is an implementation of SetFail from Superviser interface.
//
// TODO(amistewicz): WorkerList should process the reason and store it.
func (wl *WorkerList) SetFail(uuid WorkerUUID, reason string) error {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return ErrWorkerNotFound
	}
	if worker.State == MAINTENANCE {
		return ErrInMaintenance
	}
	worker.State = FAIL
	return nil
}

// SetState is an implementation of SetState from Workers interface.
func (wl *WorkerList) SetState(uuid WorkerUUID, state WorkerState) error {
	// Only state transitions to IDLE or MAINTENANCE are allowed.
	if state != MAINTENANCE && state != IDLE {
		return ErrWrongStateArgument
	}
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return ErrWorkerNotFound
	}
	// State transitions to IDLE are allowed from MAINTENANCE state only.
	if state == IDLE && worker.State != MAINTENANCE {
		return ErrForbiddenStateChange
	}
	worker.State = state
	return nil
}

// SetGroups is an implementation of SetGroups from Workers interface.
func (wl *WorkerList) SetGroups(uuid WorkerUUID, groups Groups) error {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return ErrWorkerNotFound
	}
	worker.Groups = groups
	return nil
}

// Deregister is an implementation of Deregister from Workers interface.
func (wl *WorkerList) Deregister(uuid WorkerUUID) error {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return ErrWorkerNotFound
	}
	if worker.State != MAINTENANCE {
		return ErrNotInMaintenance
	}
	delete(wl.workers, uuid)
	return nil
}

// isCapsMatching returns true if a worker has Capabilities satisfying caps.
// The worker satisfies caps if and only if one of the following statements is true:
//
// * set of required capabilities is empty,
//
// * every key present in set of required capabilities is present in set of worker's capabilities,
//
// * value of every required capability matches the value of the capability in worker.
//
// TODO Caps matching is a complex problem and it should be changed to satisfy usecases below:
// * matching any of the values and at least one:
//   "SERIAL": "57600,115200" should be satisfied by "SERIAL": "9600, 38400, 57600"
// * match value in range:
//   "VOLTAGE": "2.9-3.6" should satisfy "VOLTAGE": "3.3"
//
// It is a helper function of ListWorkers.
func isCapsMatching(worker WorkerInfo, caps Capabilities) bool {
	if len(caps) == 0 {
		return true
	}
	for srcKey, srcValue := range caps {
		destValue, found := worker.Caps[srcKey]
		if !found {
			// Key is not present in the worker's caps
			return false
		}
		if srcValue != destValue {
			// Capability values do not match
			return false
		}
	}
	return true
}

// isGroupsMatching returns true if a worker belongs to any of groups in groupsMatcher.
// Empty groupsMatcher is satisfied by every Worker.
// It is a helper function of ListWorkers.
func isGroupsMatching(worker WorkerInfo, groupsMatcher map[Group]interface{}) bool {
	if len(groupsMatcher) == 0 {
		return true
	}
	for _, workerGroup := range worker.Groups {
		_, match := groupsMatcher[workerGroup]
		if match {
			return true
		}
	}
	return false
}

// ListWorkers is an implementation of ListWorkers from Workers interface.
func (wl *WorkerList) ListWorkers(groups Groups, caps Capabilities) ([]WorkerInfo, error) {
	wl.mutex.RLock()
	defer wl.mutex.RUnlock()

	return wl.listWorkers(groups, caps)
}

// listWorkers lists all workers when both:
// * any of the groups is matching (or groups is nil)
// * all of the caps is matching (or caps is nil)
// Caller of this method should own the mutex.
func (wl *WorkerList) listWorkers(groups Groups, caps Capabilities) ([]WorkerInfo, error) {
	matching := make([]WorkerInfo, 0, len(wl.workers))

	groupsMatcher := make(map[Group]interface{})
	for _, group := range groups {
		groupsMatcher[group] = nil
	}

	for _, worker := range wl.workers {
		if isGroupsMatching(worker.WorkerInfo, groupsMatcher) &&
			isCapsMatching(worker.WorkerInfo, caps) {
			matching = append(matching, worker.WorkerInfo)
		}
	}
	return matching, nil
}

// GetWorkerInfo is an implementation of GetWorkerInfo from Workers interface.
func (wl *WorkerList) GetWorkerInfo(uuid WorkerUUID) (WorkerInfo, error) {
	wl.mutex.RLock()
	defer wl.mutex.RUnlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return WorkerInfo{}, ErrWorkerNotFound
	}
	return worker.WorkerInfo, nil
}

// SetWorkerIP stores ip in the worker structure referenced by uuid.
// It should be called after Register by function which is aware of
// the source of the connection and therefore its IP address.
func (wl *WorkerList) SetWorkerIP(uuid WorkerUUID, ip net.IP) error {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return ErrWorkerNotFound
	}
	worker.ip = ip
	return nil
}

// GetWorkerIP retrieves IP address from the internal structure.
func (wl *WorkerList) GetWorkerIP(uuid WorkerUUID) (net.IP, error) {
	wl.mutex.RLock()
	defer wl.mutex.RUnlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return nil, ErrWorkerNotFound
	}
	return worker.ip, nil
}

// SetWorkerKey stores private key in the worker structure referenced by uuid.
// It is safe to modify key after call to this function.
func (wl *WorkerList) SetWorkerKey(uuid WorkerUUID, key *rsa.PrivateKey) error {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return ErrWorkerNotFound
	}
	// Copy key so that it couldn't be changed outside this function.
	worker.key = new(rsa.PrivateKey)
	*worker.key = *key
	return nil
}

// GetWorkerKey retrieves key from the internal structure.
func (wl *WorkerList) GetWorkerKey(uuid WorkerUUID) (rsa.PrivateKey, error) {
	wl.mutex.RLock()
	defer wl.mutex.RUnlock()
	worker, ok := wl.workers[uuid]
	if !ok {
		return rsa.PrivateKey{}, ErrWorkerNotFound
	}
	return *worker.key, nil
}

// TakeBestMatchingWorker verifies which IDLE workers can satisfy Groups and
// Capabilities required by the request. Among all matched workers a best worker
// is choosen (least capable worker still fitting request). If a worker is found
// it is put into RUN state and its UUID is returned. An error is returned if no
// matching IDLE worker is found.
// It is a part of WorkersManager interface implementation by WorkerList.
func (wl *WorkerList) TakeBestMatchingWorker(groups Groups, caps Capabilities) (bestWorker WorkerUUID, err error) {
	wl.mutex.Lock()
	defer wl.mutex.Unlock()

	var bestScore = math.MaxInt32

	matching, _ := wl.listWorkers(groups, caps)
	for _, info := range matching {
		if info.State != IDLE {
			continue
		}
		score := len(info.Caps) + len(info.Groups)
		if score < bestScore {
			bestScore = score
			bestWorker = info.WorkerUUID
		}
	}
	if bestScore == math.MaxInt32 {
		err = ErrNoMatchingWorker
		return
	}

	err = wl.setState(bestWorker, RUN)
	return
}

// PrepareWorker brings worker into IDLE state and prepares it to be ready for
// running a job. In some of the situations if a worker has been matched for a job,
// but has not been used, there is no need for regeneration of the key. Caller of
// this method can decide (with 2nd parameter) if key generation is required for
// preparing worker.
//
// As key creation can take some time, the method is asynchronous and the worker's
// state might not be changed when it returns.
// It is a part of WorkersManager interface implementation by WorkerList.
func (wl *WorkerList) PrepareWorker(worker WorkerUUID, withKeyGeneration bool) error {
	if !withKeyGeneration {
		wl.mutex.Lock()
		defer wl.mutex.Unlock()
		return wl.setState(worker, IDLE)
	}

	go wl.prepareKeyAndSetState(worker)

	return nil
}

// prepareKeyAndSetState prepares private RSA key for the worker and sets worker
// into IDLE state in case of success. In case of failure of key preparation,
// worker is put into FAIL state instead.
func (wl *WorkerList) prepareKeyAndSetState(worker WorkerUUID) {
	err := wl.prepareKey(worker)
	wl.mutex.Lock()
	defer wl.mutex.Unlock()
	if err != nil {
		// TODO log error.
		wl.setState(worker, FAIL)
		return
	}
	wl.setState(worker, IDLE)
}

// setState changes state of worker. It does not contain any verification if change
// is feasible. It should be used only for internal boruta purposes. It must be
// called inside WorkerList critical section guarded by WorkerList.mutex.
func (wl *WorkerList) setState(worker WorkerUUID, state WorkerState) error {
	w, ok := wl.workers[worker]
	if !ok {
		return ErrWorkerNotFound
	}
	w.State = state
	return nil
}

// prepareKey delegates key generation to Dryad and sets up generated key in the
// worker. In case of any failure it returns an error.
func (wl *WorkerList) prepareKey(worker WorkerUUID) error {
	ip, err := wl.GetWorkerIP(worker)
	if err != nil {
		return err
	}
	client := wl.newDryadClient()
	err = client.Create(ip, conf.DefaultRPCPort)
	if err != nil {
		return err
	}
	defer client.Close()
	key, err := client.Prepare()
	if err != nil {
		return err
	}
	err = wl.SetWorkerKey(worker, key)
	return err
}

// SetChangeListener sets change listener object in WorkerList. Listener should be
// notified in case of changes of workers' states, when worker becomes IDLE
// or must break its job because of fail or maintenance.
// It is a part of WorkersManager interface implementation by WorkerList.
func (wl *WorkerList) SetChangeListener(listener WorkerChange) {
	wl.changeListener = listener
}
