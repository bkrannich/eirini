package events_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Events", func() {
	var (
		eventsConfigFile string
		eventsSession    *gexec.Session

		capiServer *ghttp.Server
		certPath   string
		keyPath    string
		logger     *lagertest.TestLogger
		config     *eirini.EventReporterConfig
	)

	BeforeEach(func() {
		var err error
		logger = lagertest.NewTestLogger("events")

		certPath, keyPath = tests.GenerateKeyPair("capi")
		capiServer, err = tests.CreateTestServer(
			certPath, keyPath, certPath,
		)
		Expect(err).NotTo(HaveOccurred())
		capiServer.HTTPTestServer.StartTLS()

		config = &eirini.EventReporterConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
			WorkloadsNamespace:      fixture.Namespace,
			CcInternalAPI:           capiServer.URL(),
			CCCertPath:              certPath,
			CCKeyPath:               keyPath,
			CCCAPath:                certPath,
			LeaderElectionID:        fmt.Sprintf("test-event-reporter-%d", GinkgoParallelNode()),
			LeaderElectionNamespace: fixture.Namespace,
		}
	})

	AfterEach(func() {
		if eventsSession != nil {
			eventsSession.Kill()
		}
		Expect(os.Remove(eventsConfigFile)).To(Succeed())
		Expect(os.Remove(certPath)).To(Succeed())
		Expect(os.Remove(keyPath)).To(Succeed())
		capiServer.Close()
	})

	JustBeforeEach(func() {
		eventsSession, eventsConfigFile = eiriniBins.EventsReporter.Run(config)
		Eventually(eventsSession).Should(gbytes.Say("Starting workers"))
	})

	Describe("LRP events", func() {
		var (
			lrpClient  *k8s.LRPClient
			lrp        opi.LRP
			lrpCommand []string
		)

		BeforeEach(func() {
			lrpToStatefulSet := stset.NewLRPToStatefulSet(
				tests.GetApplicationServiceAccount(),
				"registry-secret",
				false,
				k8s.CreateLivenessProbe,
				k8s.CreateReadinessProbe,
			)
			lrpClient = k8s.NewLRPClient(
				logger,
				client.NewSecret(fixture.Clientset),
				client.NewStatefulSet(fixture.Clientset, fixture.Namespace),
				client.NewPod(fixture.Clientset, fixture.Namespace),
				client.NewPodDisruptionBudget(fixture.Clientset),
				client.NewEvent(fixture.Clientset),
				lrpToStatefulSet,
				stset.MapStatefulSetToLRP,
			)
		})

		JustBeforeEach(func() {
			lrp = opi.LRP{
				Command:         lrpCommand,
				TargetInstances: 1,
				Image:           "eirini/busybox",
				LRPIdentifier:   opi.LRPIdentifier{GUID: tests.GenerateGUID(), Version: tests.GenerateGUID()},
			}
			Expect(lrpClient.Desire(fixture.Namespace, &lrp)).To(Succeed())
		})

		When("the LRP does not terminate", func() {
			BeforeEach(func() {
				lrpCommand = []string{"sleep", "100"}
			})

			It("should not send crash events", func() {
				Consistently(capiServer.ReceivedRequests).Should(HaveLen(0))
			})
		})

		When("the LRP terminates with code 0", func() {
			BeforeEach(func() {
				lrpCommand = []string{"/bin/sh", "-c", "exit", "0"}
			})

			JustBeforeEach(func() {
				processGUID := fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version)
				capiServer.RouteToHandler(
					"POST",
					fmt.Sprintf("/internal/v4/apps/%s/crashed", processGUID),
					func(w http.ResponseWriter, req *http.Request) {
						bytes, err := ioutil.ReadAll(req.Body)
						Expect(err).NotTo(HaveOccurred())
						request := &cc_messages.AppCrashedRequest{}
						Expect(json.Unmarshal(bytes, request)).To(Succeed())

						Expect(request.Instance).To(ContainSubstring(lrp.GUID))
						Expect(request.ExitStatus).To(Equal(0))
					},
				)
			})

			It("should generate and send a crash event", func() {
				Eventually(capiServer.ReceivedRequests).Should(HaveLen(1))
			})

			When("TLS to Cloud Controller is disabled", func() {
				var noTLSCapiServer *ghttp.Server

				BeforeEach(func() {
					noTLSCapiServer = ghttp.NewServer()
					noTLSCapiServer.AllowUnhandledRequests = true

					config = &eirini.EventReporterConfig{
						KubeConfig: eirini.KubeConfig{
							ConfigPath: fixture.KubeConfigPath,
						},
						WorkloadsNamespace:      fixture.Namespace,
						CcInternalAPI:           noTLSCapiServer.URL(),
						CCTLSDisabled:           true,
						LeaderElectionID:        fmt.Sprintf("test-event-reporter-%d", GinkgoParallelNode()),
						LeaderElectionNamespace: fixture.Namespace,
					}
				})

				AfterEach(func() {
					noTLSCapiServer.Close()
				})

				It("should generate and send a crash event", func() {
					Eventually(noTLSCapiServer.ReceivedRequests).Should(HaveLen(1))
				})
			})
		})

		When("the LRP exits with non-zero code", func() {
			BeforeEach(func() {
				lrpCommand = []string{"/bin/sh", "-c", "exit", "13"}
			})

			JustBeforeEach(func() {
				processGUID := fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version)
				capiServer.RouteToHandler(
					"POST",
					fmt.Sprintf("/internal/v4/apps/%s/crashed", processGUID),
					func(w http.ResponseWriter, req *http.Request) {
						bytes, err := ioutil.ReadAll(req.Body)
						Expect(err).NotTo(HaveOccurred())
						request := &cc_messages.AppCrashedRequest{}
						Expect(json.Unmarshal(bytes, request)).To(Succeed())

						Expect(request.Instance).To(ContainSubstring(lrp.GUID))
						Expect(request.ExitStatus).To(Equal(13))
					},
				)
			})
		})

		When("the LRP does not start because it tries to run non-existing binary", func() {
			BeforeEach(func() {
				lrpCommand = []string{"justtrolling"}
			})

			JustBeforeEach(func() {
				processGUID := fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version)
				capiServer.RouteToHandler(
					"POST",
					fmt.Sprintf("/internal/v4/apps/%s/crashed", processGUID),
					func(w http.ResponseWriter, req *http.Request) {
						bytes, err := ioutil.ReadAll(req.Body)
						Expect(err).NotTo(HaveOccurred())
						request := &cc_messages.AppCrashedRequest{}
						Expect(json.Unmarshal(bytes, request)).To(Succeed())

						Expect(request.Instance).To(ContainSubstring(lrp.GUID))
						Expect(request.ExitStatus).To(Equal(128))
					},
				)
			})

			It("should generate and send a crash event", func() {
				Eventually(capiServer.ReceivedRequests).Should(HaveLen(1))
			})
		})
	})

	Describe("Task events", func() {
		var taskDesirer jobs.Desirer

		BeforeEach(func() {
			taskToJob := jobs.NewTaskToJob(tests.GetApplicationServiceAccount(), "", false)
			taskDesirer = jobs.NewDesirer(
				logger,
				taskToJob,
				client.NewJob(fixture.Clientset, fixture.Namespace),
				nil,
			)
		})

		JustBeforeEach(func() {
			task := opi.Task{
				Command: []string{"exit", "1"},
				Image:   "eirini/busybox",
				GUID:    tests.GenerateGUID(),
			}
			Expect(taskDesirer.Desire(fixture.Namespace, &task)).To(Succeed())
		})

		It("should not send crash events", func() {
			Consistently(capiServer.ReceivedRequests).Should(HaveLen(0))
		})
	})

	Describe("Events from non-eirini pods", func() {
		BeforeEach(func() {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-0",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "potato",
							Image:   "eirini/busybox",
							Command: []string{"exit", "1"},
						},
					},
				},
			}
			_, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).Create(
				context.Background(),
				pod,
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not send crash events", func() {
			Consistently(capiServer.ReceivedRequests).Should(HaveLen(0))
		})
	})
})
