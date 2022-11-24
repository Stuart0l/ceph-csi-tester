package main

import (
	"flag"
	"fmt"

	"github.com/ceph/ceph-csi/e2e"
)

func main() {
	var seqLen int
	flag.IntVar(&seqLen, "len", 2, "test sequence length")
	flag.Parse()

	testPlanGen := e2e.NewTestPlanGen()
	// testPlanRunner := e2e.NewTestPlanRunner()

	testPlanGen.GenTestPlan(seqLen)

	fmt.Println("test plan generated, first 100 sequence:")
	for i, plan := range testPlanGen.AllPlan {
		if i > 100 {
			break
		}
		fmt.Println(plan)
	}
	fmt.Println("size:", len(testPlanGen.AllPlan))

	// for _, plan := range testPlanGen.AllPlan {
	// 	testPlanRunner.RunTestPlan(plan)
	// }
}
