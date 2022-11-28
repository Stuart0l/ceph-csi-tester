package e2e

import (
	"fmt"

	snapapi "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	clientset "k8s.io/client-go/kubernetes"
)

var (
	pvcPath        = cephfsExamplePath + "pvc.yaml"
	podPath        = cephfsExamplePath + "pod.yaml"
	snapshotPath   = cephfsExamplePath + "snapshot.yaml"
	pvcRestorePath = cephfsExamplePath + "pvc-restore.yaml"
	pvcClonePath   = cephfsExamplePath + "pvc-clone.yaml"
)

type TestPlanRunner struct {
	cli   clientset.Interface
	state runnerState
}

type runnerState struct {
	existingPVC  map[string]v1.PersistentVolumeClaim
	existingSnap map[string]snapapi.VolumeSnapshot
}

func NewTestPlanRunner() *TestPlanRunner {
	return &TestPlanRunner{}
}

func (runner *TestPlanRunner) runTestCmd(op string, param []string) error {
	switch op {
	case pvcCreate:
		pvc, err := loadPVC(pvcPath)
		pvc.Name = param[0]
		if err != nil {
			return fmt.Errorf("failed to load PVC with error %v", err)
		}
		err = createPVCAndvalidatePV(runner.cli, pvc, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create PVC with error %v", err)
		}
		runner.state.existingPVC[pvc.Name] = *pvc
	case podCreate:
		pod, err := loadApp(podPath)
		if err != nil {
			return fmt.Errorf("failed to load application with error %v", err)
		}
		pod.Name = param[0]
		pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = param[1]
		err = createApp(runner.cli, pod, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create application with error %v", err)
		}
	case snapshotCreate:
		snapshot := getSnapshot(snapshotPath)
		snapshot.Name = param[0]
		snapshot.Spec.Source.PersistentVolumeClaimName = &param[1]
		err := createSnapshot(&snapshot, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create snapshot (%s): %v", snapshot.Name, err)
		}
		runner.state.existingSnap[snapshot.Name] = snapshot
	case pvcRestoreCreate:
		pvc, err := loadPVC(pvcRestorePath)
		if err != nil {
			return fmt.Errorf("failed to load restore PVC with error %v", err)
		}
		pvc.Name = param[0]
		pvc.Spec.DataSource.Name = param[1]
		err = createPVCAndvalidatePV(runner.cli, pvc, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create restore PVC with error %v", err)
		}
		runner.state.existingPVC[pvc.Name] = *pvc
	case pvcCloneCreate:
		pvc, err := loadPVC(pvcClonePath)
		if err != nil {
			return fmt.Errorf("failed to load clone PVC with error %v", err)
		}
		pvc.Name = param[0]
		pvc.Spec.DataSource.Name = param[1]
		err = createPVCAndvalidatePV(runner.cli, pvc, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create clone PVC with error %v", err)
		}
		runner.state.existingPVC[pvc.Name] = *pvc
	case pvcDelete:
		pvc, has := runner.state.existingPVC[param[0]]
		if !has {
			return fmt.Errorf("pvc %s does not exist", param[0])
		}
		err := deletePVCAndValidatePV(runner.cli, &pvc, deployTimeout)
		delete(runner.state.existingPVC, pvc.Name)
		if err != nil {
			return fmt.Errorf("failed to delete PVC with error %v", err)
		}
	case snapshotDelete:
		snap, has := runner.state.existingSnap[param[0]]
		if !has {
			return fmt.Errorf("snapshot %s doesn not exist", param[0])
		}
		err := deleteSnapshot(&snap, deployTimeout)
		delete(runner.state.existingSnap, snap.Name)
		if err != nil {
			return fmt.Errorf("failed to delete storageclass with error %v", err)
		}
	case pvcResize:
		pvc, has := runner.state.existingPVC[param[0]]
		if !has {
			return fmt.Errorf("pvc %s does not exist", param[0])
		}
		err := expandPVCSize(runner.cli, &pvc, param[1], deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to resize PVC with error %v", err)
		}
	case resizedPvcRestoreCreate:
		pvc, err := loadPVC(pvcRestorePath)
		if err != nil {
			return fmt.Errorf("failed to load resized restore PVC with error %v", err)
		}
		pvc.Name = param[0]
		pvc.Spec.DataSource.Name = param[1]
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(param[3])
		err = createPVCAndvalidatePV(runner.cli, pvc, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create resized restore PVC with error %v", err)
		}
		runner.state.existingPVC[pvc.Name] = *pvc
	case resizedPvcCloneCreate:
		pvc, err := loadPVC(pvcClonePath)
		if err != nil {
			return fmt.Errorf("failed to load resized clone PVC with error %v", err)
		}
		pvc.Name = param[0]
		pvc.Spec.DataSource.Name = param[1]
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(param[3])
		err = createPVCAndvalidatePV(runner.cli, pvc, deployTimeout)
		if err != nil {
			return fmt.Errorf("failed to create resized clone PVC with error %v", err)
		}
		runner.state.existingPVC[pvc.Name] = *pvc
	default:
		return fmt.Errorf("unknown test op %s", op)
	}
	return nil
}

func (runner *TestPlanRunner) RunTestPlan(plan TestPlan) {
	var err error

	for _, cmd := range plan {
		if err = runner.runTestCmd(cmd.Op, cmd.Param); err != nil {
			break
		}
	}
}
