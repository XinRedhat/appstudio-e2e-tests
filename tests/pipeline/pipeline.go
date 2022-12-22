package pipeline

/* This was generated from a template file. Please feel free to update as necessary */

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"

	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	//framework imports edit as required
)

var _ = framework.PipelineSuiteDescribe("Build Pipeline-service E2E tests", Label("Pipeline", "HACBS"), func() {

	/* a couple things to note:
	   - You may need to implement specific implementation of the service/domain you are trying to test if it
	   not already there in the pkg/ packages

	   - To include the tests as part of the E2E Test suite:
	      - Update the pkg/framework/describe.go to include the `Describe func` of this new test suite
	      - Import this new package into the cmd/e2e_test.go
	*/

	defer GinkgoRecover()
	var err error
	var f *framework.Framework
	// use 'f' to access common controllers or the specific service controllers within the framework
	BeforeAll(func() {
		// Initialize the tests controllers
		f, err = framework.NewFramework()
		Expect(err).NotTo(HaveOccurred())
	})

	/* In Gingko, "Describe", "Context", "When" are functionaly the same. They are container nodes that hierarchically
	   organize the specs, used to make the flow read better. The core piece of the spec is the subject container, "It",
	   this is where the meat of the test is written.

	   Ginkgo's default behavior is to only randomize the order of top-level containers -- the specs within those containers
	   continue to run in the order in which they are specified in the test files. That being said, Ginko does provide the
	   option to randomize ALL specs. So it is important to design and write test cases for randomization and parallelization
	   in mind. Tips to do so:

	   - Declare variables in "Describe" containers, initialize in "BeforeEach/All" containers
	   - Move all setup code into "BeforeEach/All" closures
	   - "It" containers should be independent of each other. If you require the "It" tests to be dependent on each other for
	     complex test scenarios, append into the "Describe" the "Ordered" decorator.
	*/

	Describe("Pipeline-service is working as expected", Label("Pipeline"), func() {
		// Declare variables here.
		var pipelineRunName, testNamespace string

		BeforeEach(func() {
			url := "https://raw.githubusercontent.com/tektoncd/pipeline/v0.32.0/examples/v1beta1/pipelineruns/using_context_variables.yaml"
			obj, err := tekton.StreamRemoteYamlToTektonObj(url, &v1beta1.PipelineRun{})
			Expect(err).ShouldNot(HaveOccurred())

			pipelineRun, ok := obj.(*v1beta1.PipelineRun)
			if !ok {
				Fail("Failed parse pipelinerun yaml file")
			}
			// change ubuntu image to ubi to avoid dockerhub registry pull limit
			tasks := &pipelineRun.Spec.PipelineSpec.Tasks[0].TaskSpec.Steps
			for i := range *tasks {
				task := *tasks
				task[i].Image = "registry.access.redhat.com/ubi9/ubi-minimal:latest"
			}
			pipelineRun, err = f.TektonController.CreatePipelineRun(pipelineRun, "pipeline")
			Expect(err).ShouldNot(HaveOccurred())
			pipelineRunName = pipelineRun.GetObjectMeta().GetName()
		})

		It("Verify PipelineRun complete successfully", func() {
			err = utils.WaitUntil(f.TektonController.CheckPipelineRunFinished(pipelineRunName, testNamespace), time.Duration(10)*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(f.CommonController.DeleteNamespace(testNamespace)).Should(Succeed())
		})

	})

})
