package pipeline

/* This was generated from a template file. Please feel free to update as necessary */

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"knative.dev/pkg/apis"

	"github.com/redhat-appstudio/e2e-tests/pkg/apis/github"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
)

const (
	sourceOwner  = "devfile-samples"
	sourceRepo   = "devfile-sample-go-basic"
	mainBranch   = "main"
	pacNamespace = "" // This is the namespace that is created for running pipeilneruns triggered by Github App
)

var _ = framework.PipelineSuiteDescribe("Pipeline E2E tests", Label("pipeline"), func() {
	var f *framework.Framework
	var err error

	defer GinkgoRecover()

	Describe("Test the Pac component", Label("pipeline"), func() {

		var testNamespace, forkedRepo, newBranch string
		var gh *github.Github

		BeforeAll(func() {
			f, err = framework.NewFramework(utils.GetGeneratedNamespace("pac-e2e"))
			Expect(err).NotTo(HaveOccurred())
			testNamespace = f.UserNamespace
			// Fork a repo
			gh = f.AsKubeAdmin.CommonController.Github
			//TODO:
			forkedRepo = ""
			newBranch = ""
			repo, err := gh.ForkRepository(sourceOwner, sourceRepo, forkedRepo)
			Expect(err).NotTo(HaveOccurred())
			Expect(gh.CheckIfRepositoryExist(repo.GetName())).To(BeTrue())
			// Create a new branch
			err = gh.CreateRef(forkedRepo, mainBranch, newBranch)
			Expect(err).NotTo(HaveOccurred())
			// update file content in the new branch
			_, err = gh.CreateFile(forkedRepo, "test.txt", "Added a new file", newBranch)
			Expect(err).NotTo(HaveOccurred())
			// Create a PR
			_, err = gh.CreatePullRequest(forkedRepo, newBranch, mainBranch)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				Expect(f.AsKubeAdmin.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
			}
		})

		It("Should triggers a PipelineRun", func() {
			timeout := time.Second * 30
			interval := time.Second * 5
			Eventually(func() bool {
				pipelineRunList, err := f.AsKubeAdmin.TektonController.ListAllPipelineRuns(pacNamespace)
				if err != nil || len(pipelineRunList.Items) == 0 {
					return false
				}
				// If there is a pipeinerun triggered, we suppose only one pipeinerun under this namespace
				pipelineRun := &pipelineRunList.Items[0]
				return pipelineRun.HasStarted()
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for PipelineRun to start")
		})

		It("There should be CI checkes in Github page", func() {

		})

		It("The PipelineRun should eventually finish successfully", func() {
			timeout := time.Second * 120
			interval := time.Second * 5
			Eventually(func() bool {
				pipelineRunList, err := f.AsKubeAdmin.TektonController.ListAllPipelineRuns(pacNamespace)
				if err != nil {
					return false
				}
				// If there is a pipeinerun triggered, we suppose only one pipeinerun under this namespace
				pipelineRun := &pipelineRunList.Items[0]
				for _, condition := range pipelineRun.Status.Conditions {
					GinkgoWriter.Printf("PipelineRun %s Status.Conditions.Reason: %s\n", pipelineRun.Name, condition.Reason)

					if !pipelineRun.IsDone() {
						return false
					}

					if !pipelineRun.GetStatusCondition().GetCondition(apis.ConditionSucceeded).IsTrue() {
						failMessage := tekton.GetFailedPipelineRunLogs(f.AsKubeAdmin.CommonController, pipelineRun)
						Fail(failMessage)
					}
				}
				return true
			}, timeout, interval).Should(BeTrue(), "timed out when waiting for PipelineRun to start")
		})

	})

})
