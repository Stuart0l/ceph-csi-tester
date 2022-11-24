package e2e

import (
	"strconv"
)

// operations
const (
	pvcCreate               = "createPvc"
	podCreate               = "createPod"
	snapshotCreate          = "createSnapshot"
	pvcRestoreCreate        = "createPvcRestore"
	pvcCloneCreate          = "createPvcClone"
	pvcDelete               = "deletePvc"
	snapshotDelete          = "deleteSnapshot"
	pvcResize               = "resizePvc"
	resizedPvcRestoreCreate = "createResizedPvcRestore"
	resizedPvcCloneCreate   = "createResizedPvcClone"
)

var (
	pvcName        = "csi-cephfs-pvc"
	podName        = "csi-cephfs-demo-pod"
	snapshotName   = "cephfs-pvc-snapshot"
	pvcRestoreName = "cephfs-pvc-restore"
	pvcCloneName   = "cephfs-pvc-clone"
)

type TestCmd struct {
	Op    string
	Param []string
}

type TestPlan []TestCmd

var Exist = struct{}{}

type Set map[interface{}]struct{}

type testState struct {
	PvcPodMapping      map[string]string
	PvcSnapMapping     map[string]string
	SnapRestoreMapping map[string]string
	PvcCloneMapping    map[string]string
	IsPvcRestore       map[string]bool
	IsPvcClone         map[string]bool
	IsPvcResized       map[string]bool
	PvcSet             Set
	PodSet             Set
	SnapSet            Set
}

type TestPlanGen struct {
	Plan    TestPlan
	state   testState
	max_len int
	AllPlan []TestPlan
}

func (set Set) GetStrElem() []string {
	elem := []string{}
	for e := range set {
		elem = append(elem, e.(string))
	}
	return elem
}

func (plan TestPlan) Copy() TestPlan {
	planCopy := make(TestPlan, len(plan))
	copy(planCopy, plan)
	return planCopy
}

/// all test plan should start with creating a pvc
func NewTestPlanGen() *TestPlanGen {
	initPlan := []TestCmd{{
		Op:    pvcCreate,
		Param: []string{pvcName},
	}}

	initState := testState{
		PvcPodMapping:      make(map[string]string),
		PvcSnapMapping:     make(map[string]string),
		SnapRestoreMapping: make(map[string]string),
		PvcCloneMapping:    make(map[string]string),
		IsPvcRestore:       make(map[string]bool),
		IsPvcClone:         make(map[string]bool),
		IsPvcResized:       make(map[string]bool),
		PvcSet: Set{
			pvcName: Exist,
		},
		PodSet:  make(Set),
		SnapSet: make(Set),
	}

	return &TestPlanGen{
		Plan:  initPlan,
		state: initState,
	}
}

func (gen *TestPlanGen) pushTestCmd(test string, param []string) {
	gen.Plan = append(gen.Plan, TestCmd{
		Op:    test,
		Param: param,
	})
}

func (gen *TestPlanGen) popTestCmd() {
	len := len(gen.Plan)
	if len > 0 {
		gen.Plan = gen.Plan[:len-1]
	}
}

/// create a pod to consume a pvc, iff the pvc is not consumed by another pod
/// we assume that each pvc is only consumed by one pod
func (gen *TestPlanGen) addPodCreate(pod string, pvc string) bool {
	if _, ok := gen.state.PvcPodMapping[pvc]; ok {
		return false
	}
	gen.state.PvcPodMapping[pvc] = pod
	gen.state.PodSet[pod] = Exist

	// createPod(pod, pvc)
	gen.pushTestCmd(podCreate, []string{pod, pvc})
	return true
}

func (gen *TestPlanGen) rmPodCreate(pod string, pvc string) {
	delete(gen.state.PvcPodMapping, pvc)
	delete(gen.state.PodSet, pod)
	gen.popTestCmd()
}

/// create a snapshot from a pvc, iff no snapshot has been created from this pvc
/// we assume that at most one snapshot will be created from a pvc,
/// and we don't create snapshot for a restored pvc
func (gen *TestPlanGen) addSnapshotCreate(snapshot string, pvc string) bool {
	_, hasSnapshot := gen.state.PvcSnapMapping[pvc]
	_, isRestore := gen.state.IsPvcRestore[pvc]
	if hasSnapshot || isRestore {
		return false
	}
	gen.state.PvcSnapMapping[pvc] = snapshot
	gen.state.SnapSet[snapshot] = Exist

	// createSnapshot(snapshot, pvc)
	gen.pushTestCmd(snapshotCreate, []string{snapshot, pvc})
	return true
}

func (gen *TestPlanGen) rmSnapshotCreate(snapshot string, pvc string) {
	delete(gen.state.PvcSnapMapping, pvc)
	delete(gen.state.SnapSet, snapshot)
	gen.popTestCmd()
}

/// create a restored pvc from a snapshot
/// we assume at most one restored pvc will be created from a snapshot
func (gen *TestPlanGen) addPvcRestoreCreate(restorePvc string, snapshot string) bool {
	if _, ok := gen.state.SnapRestoreMapping[snapshot]; ok {
		return false
	}
	gen.state.SnapRestoreMapping[snapshot] = restorePvc
	gen.state.IsPvcRestore[restorePvc] = true // to prevent creating snapshot from the restored pvc
	gen.state.PvcSet[restorePvc] = Exist

	// createPvcRestore(restorePvc, snapshot)
	gen.pushTestCmd(pvcRestoreCreate, []string{restorePvc, snapshot})
	return true
}

func (gen *TestPlanGen) rmPvcRestoreCreate(restorePvc string, snapshot string) {
	delete(gen.state.SnapRestoreMapping, snapshot)
	delete(gen.state.IsPvcRestore, restorePvc)
	delete(gen.state.PvcSet, restorePvc)
	gen.popTestCmd()
}

/// create a cloned pvc from a pvc
/// we assume at most one cloned pvc will be create from a pvc,
/// and we don't create a clone from a cloned pvc
func (gen *TestPlanGen) addPvcCloneCreate(clonePvc string, pvc string) bool {
	_, hasClone := gen.state.PvcCloneMapping[pvc]
	_, isClone := gen.state.IsPvcClone[pvc]
	if hasClone || isClone {
		return false
	}

	gen.state.PvcCloneMapping[pvc] = clonePvc
	gen.state.IsPvcClone[clonePvc] = true

	// createPvcClone(clonePvc, pvc)
	gen.pushTestCmd(pvcCloneCreate, []string{clonePvc, pvc})
	gen.state.PvcSet[clonePvc] = Exist
	return true
}

func (gen *TestPlanGen) rmPvcCloneCreate(clonePvc string, pvc string) {
	delete(gen.state.PvcCloneMapping, pvc)
	delete(gen.state.IsPvcClone, clonePvc)
	delete(gen.state.PvcSet, clonePvc)
	gen.popTestCmd()
}

/// delete a existing pvc
func (gen *TestPlanGen) addPvcDelete(pvc string) {
	delete(gen.state.PvcSet, pvc)

	// deletePvc(pvc)
	gen.pushTestCmd(pvcDelete, []string{pvc})
}

func (gen *TestPlanGen) rmPvcDelete(pvc string) {
	gen.state.PvcSet[pvc] = Exist
	gen.popTestCmd()
}

/// delete a existing snapshot
func (gen *TestPlanGen) addSnapshotDelete(snapshot string) {
	delete(gen.state.SnapSet, snapshot)

	// deleteSnapshot(snapshot)
	gen.pushTestCmd(snapshotDelete, []string{snapshot})
}

func (gen *TestPlanGen) rmSnapshotDelete(snapshot string) {
	gen.state.SnapSet[snapshot] = Exist
	gen.popTestCmd()
}

/// resize a existing pvc
/// we assume a pvc will be resized at most once,
/// and we don't resize a restored or cloned pvc (as we can create a resized clone or resized restore)
func (gen *TestPlanGen) addPvcResize(pvc string, resize_g int) bool {
	_, isClone := gen.state.IsPvcClone[pvc]
	_, isRestore := gen.state.IsPvcRestore[pvc]
	_, isResized := gen.state.IsPvcResized[pvc]

	if isClone || isResized || isRestore {
		return false
	}
	gen.state.IsPvcResized[pvc] = true

	// resizePvc(pvc)
	gen.pushTestCmd(pvcResize, []string{pvc, strconv.Itoa(resize_g)})
	return true
}

func (gen *TestPlanGen) rmPvcResize(pvc string) {
	delete(gen.state.IsPvcResized, pvc)
	gen.popTestCmd()
}

/// create a resized restored pvc from a snapshot
func (gen *TestPlanGen) addResizePvcRestore(resizedRestorePvc string, snapshot string, resize_g int) bool {
	if _, ok := gen.state.SnapRestoreMapping[snapshot]; ok {
		return false
	}
	gen.state.SnapRestoreMapping[snapshot] = resizedRestorePvc
	gen.state.IsPvcRestore[resizedRestorePvc] = true // to prevent creating snapshot from the restored pvc
	gen.state.PvcSet[resizedRestorePvc] = Exist

	// createResizedPvcRestore(resizeRestorePvc, snapshot, resize_g)
	gen.pushTestCmd(resizedPvcRestoreCreate, []string{resizedRestorePvc, snapshot, strconv.Itoa(resize_g)})
	return true
}

func (gen *TestPlanGen) rmResizedPvcRestore(resizedRestorePvc string, snapshot string) {
	delete(gen.state.SnapRestoreMapping, snapshot)
	delete(gen.state.IsPvcRestore, resizedRestorePvc)
	delete(gen.state.PvcSet, resizedRestorePvc)
	gen.popTestCmd()
}

/// create a resized cloned pvc from a pvc
func (gen *TestPlanGen) addResizePvcClone(resizedClonePvc string, pvc string, resize_g int) bool {
	_, hasClone := gen.state.PvcCloneMapping[pvc]
	_, isClone := gen.state.IsPvcClone[pvc]
	if hasClone || isClone {
		return false
	}

	gen.state.PvcCloneMapping[pvc] = resizedClonePvc
	gen.state.IsPvcClone[resizedClonePvc] = true

	// createResizedPvcClone(clonePvc, pvc, resize_g)
	gen.pushTestCmd(resizedPvcCloneCreate, []string{resizedClonePvc, pvc, strconv.Itoa(resize_g)})
	gen.state.PvcSet[resizedClonePvc] = Exist
	return true
}

func (gen *TestPlanGen) rmResizePvcClone(resizedClonePvc string, pvc string) {
	delete(gen.state.PvcCloneMapping, pvc)
	delete(gen.state.IsPvcClone, resizedClonePvc)
	delete(gen.state.PvcSet, resizedClonePvc)
	gen.popTestCmd()
}

func (gen *TestPlanGen) GenTestPlan(max_len int) {
	gen.max_len = max_len
	gen.genTestPlanBacktrack()
}

func (gen *TestPlanGen) genTestPlanBacktrack() {
	if len(gen.Plan) == gen.max_len {
		// fmt.Println(gen.Plan)
		gen.AllPlan = append(gen.AllPlan, gen.Plan.Copy())
		return
	}

	for _, pvc := range gen.state.PvcSet.GetStrElem() {
		if gen.addPodCreate(podName, pvc) {
			gen.genTestPlanBacktrack()
			gen.rmPodCreate(podName, pvc)
		}
	}

	for _, pvc := range gen.state.PvcSet.GetStrElem() {
		if gen.addSnapshotCreate(snapshotName, pvc) {
			gen.genTestPlanBacktrack()
			gen.rmSnapshotCreate(snapshotName, pvc)
		}
	}

	for _, snapshot := range gen.state.SnapSet.GetStrElem() {
		if gen.addPvcRestoreCreate(pvcRestoreName, snapshot) {
			gen.genTestPlanBacktrack()
			gen.rmPvcRestoreCreate(pvcRestoreName, snapshot)
		}
	}

	for _, pvc := range gen.state.PvcSet.GetStrElem() {
		if gen.addPvcCloneCreate(pvcCloneName, pvc) {
			gen.genTestPlanBacktrack()
			gen.rmPvcCloneCreate(pvcCloneName, pvc)
		}
	}

	for _, pvc := range gen.state.PvcSet.GetStrElem() {
		gen.addPvcDelete(pvc)
		gen.genTestPlanBacktrack()
		gen.rmPvcDelete(pvc)
	}

	for _, snapshot := range gen.state.SnapSet.GetStrElem() {
		gen.addSnapshotDelete(snapshot)
		gen.genTestPlanBacktrack()
		gen.rmSnapshotDelete(snapshot)
	}

	for _, pvc := range gen.state.PvcSet.GetStrElem() {
		if gen.addPvcResize(pvc, 5) {
			gen.genTestPlanBacktrack()
			gen.rmPvcResize(pvc)
		}
	}

	for _, snapshot := range gen.state.SnapSet.GetStrElem() {
		if gen.addResizePvcRestore(pvcRestoreName, snapshot, 5) {
			gen.genTestPlanBacktrack()
			gen.rmResizedPvcRestore(pvcRestoreName, snapshot)
		}
	}

	for _, pvc := range gen.state.PvcSet.GetStrElem() {
		if gen.addResizePvcClone(pvcCloneName, pvc, 5) {
			gen.genTestPlanBacktrack()
			gen.rmResizePvcClone(pvcCloneName, pvc)
		}
	}
}
