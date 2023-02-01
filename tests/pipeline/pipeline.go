package pipeline

/* This was generated from a template file. Please feel free to update as necessary */

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devfile/library/pkg/util"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	"github.com/redhat-appstudio/e2e-tests/pkg/constants"
	"github.com/redhat-appstudio/e2e-tests/pkg/framework"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils"
	"github.com/redhat-appstudio/e2e-tests/pkg/utils/tekton"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	helloWorldComponentGitSourceRepoName = "devfile-sample-hello-world"
	pythonComponentGitSourceURL          = "https://github.com/redhat-appstudio-qe/devfile-sample-python-basic.git"
)

var (
	componentUrls  = strings.Split(utils.GetEnv(COMPONENT_REPO_URLS_ENV, pythonComponentGitSourceURL), ",") //multiple urls
	componentNames []string
)

var _ = framework.PipelineSuiteDescribe("Pipeline E2E tests", Label("pipeline"), func() {
	pipelineRunTimeout := 360

	defer GinkgoRecover()
	var fwk *framework.Framework
	// use 'fwk' to access common controllers or the specific service controllers within the framework
	BeforeAll(func() {
		// Initialize the tests controllers
		var err error
		fwk, err = framework.NewFramework()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Trigger PipelineRun directly by calling Pipeline-service", Label("pipeline"), func() {
		// Declare variables here.
		namespace := constants.PIPELINE_SERVICE_E2E_NS
		buildPipelineRunName := fmt.Sprintf("buildah-demo-%s", util.GenerateRandomString(10))
		image := fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/%s", namespace, buildPipelineRunName)
		var imageWithDigest string
		serviceAccountName := "pipeline"

		attestationTimeout := time.Duration(60) * time.Second

		var kubeController tekton.KubeController

		BeforeAll(func() {
			kubeController = tekton.KubeController{
				Commonctrl: *fwk.CommonController,
				Tektonctrl: *fwk.TektonController,
				Namespace:  namespace,
			}
			// Create the e2e test namespace
			_, err := kubeController.Commonctrl.CreateTestNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating namespace %q: %v", namespace, err)

			// Wait until the "pipeline" SA is created
			GinkgoWriter.Printf("Wait until the %q SA is created in namespace %q\n", serviceAccountName, namespace)
			Eventually(func() bool {
				sa, err := kubeController.Commonctrl.GetServiceAccount(serviceAccountName, namespace)
				return sa != nil && err == nil
			}).WithTimeout(1*time.Minute).WithPolling(100*time.Millisecond).Should(
				BeTrue(), "timed out when waiting for the %q SA to be created", serviceAccountName)

			// At a bare minimum, each spec within this context relies on the existence of
			// an image that has been signed by Tekton Chains. Trigger a demo task to fulfill
			// this purpose.
			pr, err := kubeController.RunPipeline(tekton.BuildahDemo{Image: image, Bundle: fwk.TektonController.Bundles.BuildTemplatesBundle}, pipelineRunTimeout)
			Expect(err).NotTo(HaveOccurred())
			// Verify that the pipelinerun is executed as expected.
			Expect(pr.ObjectMeta.Name).To(Equal(buildPipelineRunName))
			Expect(pr.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(kubeController.WatchPipelineRun(pr.Name, pipelineRunTimeout)).To(Succeed())
			GinkgoWriter.Printf("The pipeline named %q in namespace %q succeeded\n", pr.ObjectMeta.Name, pr.ObjectMeta.Namespace)

			// The PipelineRun resource has been updated, refresh our reference.
			pr, err = kubeController.Tektonctrl.GetPipelineRun(pr.ObjectMeta.Name, pr.ObjectMeta.Namespace)
			Expect(err).NotTo(HaveOccurred())

			// Verify TaskRun has the type hinting required by Tekton Chains
			digest, err := kubeController.GetTaskRunResult(pr, "build-container", "IMAGE_DIGEST")
			Expect(err).NotTo(HaveOccurred())
			i, err := kubeController.GetTaskRunResult(pr, "build-container", "IMAGE_URL")
			Expect(err).NotTo(HaveOccurred())
			Expect(i).To(Equal(image))

			// Specs now have a deterministic image reference for validation \o/
			imageWithDigest = fmt.Sprintf("%s@%s", image, digest)

			GinkgoWriter.Printf("The image signed by Tekton Chains is %s\n", imageWithDigest)
		})

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				Expect(fwk.TektonController.DeleteAllPipelineRunsInASpecificNamespace(namespace)).To(Succeed())
				Expect(fwk.CommonController.DeleteNamespace(namespace)).To(Succeed())
			}
		})

		Context("Test Tekton Chanin", func() {
			It("creates signature and attestation", func() {
				err := kubeController.AwaitAttestationAndSignature(imageWithDigest, attestationTimeout)
				Expect(err).NotTo(
					HaveOccurred(),
					"Could not find .att or .sig ImageStreamTags within the %s timeout. "+
						"Most likely the chains-controller did not create those in time. "+
						"Look at the chains-controller logs.",
					attestationTimeout.String(),
				)
				GinkgoWriter.Printf("Cosign verify pass with .att and .sig ImageStreamTags found for %s\n", imageWithDigest)
			})
		})

		Context("Test Tekton Result ", func() {
			result_namespace := "tekton-results"
			test_sa := "tekton-results-debug"
			It("Verify Pipeline result is stored in persistent storage", func() {
				//Download the API Server certificate locally
				certPath := "/tmp/results.crt"
				secret, err := fwk.CommonController.GetSecret(result_namespace, "tekton-results-tls")
				Expect(err).ToNot(HaveOccurred())
				os.WriteFile(certPath, secret.Data[v1.TLSCertKey], 0644)

				//Retrieve Access Token
				testSA := &v1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: "tekton-results-debug"},
				}
				fwk.CommonController.CreateServiceAccount(result_namespace, testSA)
				// Wait until the "tekton-results-debug" SA is created
				GinkgoWriter.Printf("Wait until the %q SA is created in namespace %q\n", testSA.GetName(), result_namespace)
				Eventually(func() bool {
					sa, err := kubeController.Commonctrl.GetServiceAccount(serviceAccountName, namespace)
					return sa != nil && err == nil
				}).WithTimeout(1*time.Minute).WithPolling(100*time.Millisecond).Should(
					BeTrue(), "timed out when waiting for the %q SA to be created", serviceAccountName)

				testClusterRoleBinding := &rbacv1.ClusterRoleBinding{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{Name: test_sa},
					Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: namespace, Namespace: namespace}},
					RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "tekton-results-readonly"},
				}
				fwk.CommonController.CreateClusterRoleBinding(testClusterRoleBinding)

				accessToken, err := fwk.CommonController.GetServiceAccountToken(result_namespace, test_sa)
				Expect(err).NotTo(HaveOccurred())
				fmt.Println("accessToken: ", accessToken)
				//
				// tekton.NewResultsClient(certPath)
				//Retrieve result ID from Pipelinerun
				pr, err := kubeController.Tektonctrl.GetPipelineRun(buildPipelineRunName, namespace)
				Expect(err).NotTo(HaveOccurred())
				resultUID := pr.ObjectMeta.UID
				fmt.Println("resultUID: ", resultUID)
				//Retrieve result record from persistent storage

			})
		})
	})

	Describe("Trigger PipelineRun by Creating Component CR", Ordered, Label("pipeline"), func() {
		var applicationName, componentName, testNamespace, outputContainerImage string
		var kubeController tekton.KubeController
		BeforeAll(func() {
			if os.Getenv("APP_SUFFIX") != "" {
				applicationName = fmt.Sprintf("test-app-%s", os.Getenv("APP_SUFFIX"))
			} else {
				applicationName = fmt.Sprintf("test-app-%s", util.GenerateRandomString(4))
			}
			testNamespace = utils.GetEnv(constants.E2E_APPLICATIONS_NAMESPACE_ENV, fmt.Sprintf("pipeline-e2e-%s", util.GenerateRandomString(4)))

			kubeController = tekton.KubeController{
				Commonctrl: *fwk.CommonController,
				Tektonctrl: *fwk.TektonController,
				Namespace:  testNamespace,
			}

			_, err := fwk.CommonController.CreateTestNamespace(testNamespace)
			Expect(err).NotTo(HaveOccurred(), "Error when creating/updating '%s' namespace: %v", testNamespace, err)

			_, err = fwk.HasController.GetHasApplication(applicationName, testNamespace)
			// In case the app with the same name exist in the selected namespace, delete it first
			if err == nil {
				Expect(fwk.HasController.DeleteHasApplication(applicationName, testNamespace, false)).To(Succeed())
				Eventually(func() bool {
					_, err := fwk.HasController.GetHasApplication(applicationName, testNamespace)
					return errors.IsNotFound(err)
				}, time.Minute*5, time.Second*1).Should(BeTrue(), "timed out when waiting for the app %s to be deleted in %s namespace", applicationName, testNamespace)
			}
			app, err := fwk.HasController.CreateHasApplication(applicationName, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(utils.WaitUntil(fwk.CommonController.ApplicationGitopsRepoExists(app.Status.Devfile), 30*time.Second)).To(
				Succeed(), fmt.Sprintf("timed out waiting for gitops content to be created for app %s in namespace %s: %+v", app.Name, app.Namespace, err),
			)

			for _, gitUrl := range componentUrls {
				gitUrl := gitUrl
				componentName = fmt.Sprintf("%s-%s", "test-component", util.GenerateRandomString(4))
				componentNames = append(componentNames, componentName)
				outputContainerImage = fmt.Sprintf("quay.io/%s/test-images:%s", utils.GetQuayIOOrganization(), strings.Replace(uuid.New().String(), "-", "", -1))
				// Create a component with Git Source URL being defined
				_, err := fwk.HasController.CreateComponent(applicationName, componentName, testNamespace, gitUrl, "", "", outputContainerImage, "", false)
				Expect(err).ShouldNot(HaveOccurred())
			}
		})

		AfterAll(func() {
			// Do cleanup only in case the test succeeded
			if !CurrentSpecReport().Failed() {
				// Clean up only Application CR (Component and Pipelines are included) in case we are targeting specific namespace
				// Used e.g. in build-definitions e2e tests, where we are targeting build-templates-e2e namespace
				if os.Getenv(constants.E2E_APPLICATIONS_NAMESPACE_ENV) != "" {
					DeferCleanup(fwk.HasController.DeleteHasApplication, applicationName, testNamespace, false)
				} else {
					Expect(fwk.TektonController.DeleteAllPipelineRunsInASpecificNamespace(testNamespace)).To(Succeed())
					Expect(fwk.CommonController.DeleteNamespace(testNamespace)).To(Succeed())
				}
			}
		})

		for i, gitUrl := range componentUrls {
			gitUrl := gitUrl
			It(fmt.Sprintf("triggers PipelineRun for component with source URL %s", gitUrl), func() {
				timeout := time.Minute * 5
				interval := time.Second * 1

				Eventually(func() bool {
					pipelineRun, err := fwk.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, false, "")
					if err != nil {
						GinkgoWriter.Println("PipelineRun has not been created yet")
						return false
					}
					return pipelineRun.HasStarted()
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the %s PipelineRun to start", componentNames[i])
			})
		}

		for i, gitUrl := range componentUrls {
			gitUrl := gitUrl
			It(fmt.Sprintf("should eventually finish successfully for component with source URL %s", gitUrl), func() {
				timeout := time.Second * 900
				interval := time.Second * 10
				Eventually(func() bool {
					pipelineRun, err := fwk.HasController.GetComponentPipelineRun(componentNames[i], applicationName, testNamespace, false, "")
					Expect(err).ShouldNot(HaveOccurred())

					err = kubeController.WatchPipelineRun(pipelineRun.Name, pipelineRunTimeout)
					return err == nil
				}, timeout, interval).Should(BeTrue(), "timed out when waiting for the PipelineRun to finish")
			})

			// It("should validate HACBS taskrun results", func() {
			// 	// List Of Taskruns Expected to Get Taskrun Results
			// 	gatherResult := []string{"conftest-clair", "sanity-inspect-image", "sanity-label-check"}
			// 	// TODO: once we migrate "build" e2e tests to kcp, remove this condition
			// 	// and add the 'sbom-json-check' taskrun to gatherResults slice
			// 	s, _ := GinkgoConfiguration()
			// 	if strings.Contains(s.LabelFilter, buildTemplatesKcpTestLabel) {
			// 		gatherResult = append(gatherResult, "sbom-json-check")
			// 	}
			// 	pipelineRun, err := f.HasController.GetComponentPipelineRun(componentNames[0], applicationName, testNamespace, false, "")
			// 	Expect(err).ShouldNot(HaveOccurred())

			// 	for i := range gatherResult {
			// 		if gatherResult[i] == "sanity-inspect-image" {
			// 			result, err := build.FetchImageTaskRunResult(pipelineRun, gatherResult[i], "BASE_IMAGE")
			// 			Expect(err).ShouldNot(HaveOccurred())
			// 			ret := build.ValidateImageTaskRunResults(gatherResult[i], result)
			// 			Expect(ret).Should(BeTrue())
			// 			// TODO conftest-clair returns SUCCESS which is not expected
			// 			// } else {
			// 			// 	result, err := build.FetchTaskRunResult(pipelineRun, gatherResult[i], "HACBS_TEST_OUTPUT")
			// 			// 	Expect(err).ShouldNot(HaveOccurred())
			// 			// 	ret := build.ValidateTaskRunResults(gatherResult[i], result)
			// 			// 	Expect(ret).Should(BeTrue())
			// 		}
			// 	}
			// })
		}
	})
})
