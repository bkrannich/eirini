package bifrost_test

import (
	"context"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("Task", func() {
	var (
		err           error
		taskBifrost   *bifrost.Task
		taskConverter *bifrostfakes.FakeTaskConverter
		taskClient    *bifrostfakes.FakeTaskClient
		jsonClient    *bifrostfakes.FakeJSONClient
		namespacer    *bifrostfakes.FakeTaskNamespacer
		taskGUID      string
		task          opi.Task
	)

	BeforeEach(func() {
		taskConverter = new(bifrostfakes.FakeTaskConverter)
		taskClient = new(bifrostfakes.FakeTaskClient)
		jsonClient = new(bifrostfakes.FakeJSONClient)
		namespacer = new(bifrostfakes.FakeTaskNamespacer)

		taskGUID = "task-guid"
		task = opi.Task{GUID: "my-guid"}
		taskConverter.ConvertTaskReturns(task, nil)
		namespacer.GetNamespaceReturns("our-namespace")

		taskBifrost = &bifrost.Task{
			Converter:  taskConverter,
			TaskClient: taskClient,
			JSONClient: jsonClient,
			Namespacer: namespacer,
		}
	})

	Describe("Transfer Task", func() {
		var taskRequest cf.TaskRequest

		BeforeEach(func() {
			taskRequest = cf.TaskRequest{
				Name:               "cake",
				AppGUID:            "app-guid",
				AppName:            "foo",
				OrgName:            "my-org",
				OrgGUID:            "asdf123",
				SpaceName:          "my-space",
				SpaceGUID:          "fdsa4321",
				Namespace:          "my-namespace",
				CompletionCallback: "my-callback",
				Environment:        nil,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{},
				},
			}
			task := opi.Task{GUID: "my-guid"}
			taskConverter.ConvertTaskReturns(task, nil)
		})

		JustBeforeEach(func() {
			err = taskBifrost.TransferTask(context.Background(), taskGUID, taskRequest)
		})

		It("transfers the task", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(taskConverter.ConvertTaskCallCount()).To(Equal(1))
			actualTaskGUID, actualTaskRequest := taskConverter.ConvertTaskArgsForCall(0)
			Expect(actualTaskGUID).To(Equal(taskGUID))
			Expect(actualTaskRequest).To(Equal(taskRequest))

			Expect(taskClient.DesireCallCount()).To(Equal(1))
			namespace, desiredTask, _ := taskClient.DesireArgsForCall(0)
			Expect(desiredTask.GUID).To(Equal("my-guid"))
			Expect(namespace).To(Equal("our-namespace"))
		})

		When("converting the task fails", func() {
			BeforeEach(func() {
				taskConverter.ConvertTaskReturns(opi.Task{}, errors.New("task-conv-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("task-conv-err")))
			})

			It("does not desire the task", func() {
				Expect(taskClient.DesireCallCount()).To(Equal(0))
			})
		})

		When("desiring the task fails", func() {
			BeforeEach(func() {
				taskClient.DesireReturns(errors.New("desire-task-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("desire-task-err")))
			})
		})
	})

	Describe("GetTask", func() {
		var taskResponse cf.TaskResponse

		BeforeEach(func() {
			taskClient.GetReturns(&opi.Task{GUID: taskGUID}, nil)
		})

		JustBeforeEach(func() {
			taskResponse, err = taskBifrost.GetTask(taskGUID)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("finds a task by GUID", func() {
			Expect(taskClient.GetCallCount()).To(Equal(1))
			Expect(taskClient.GetArgsForCall(0)).To(Equal(taskGUID))
			Expect(taskResponse.GUID).To(Equal(taskGUID))
		})

		When("finding the task fails", func() {
			BeforeEach(func() {
				taskClient.GetReturns(nil, errors.New("task-error"))
			})

			It("fails", func() {
				Expect(err).To(MatchError(ContainSubstring("task-error")))
			})
		})
	})

	Describe("ListTasks", func() {
		var tasksResponse cf.TasksResponse

		BeforeEach(func() {
			taskClient.ListReturns([]*opi.Task{{GUID: taskGUID}}, nil)
		})

		JustBeforeEach(func() {
			tasksResponse, err = taskBifrost.ListTasks()
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("lists all tasks", func() {
			Expect(taskClient.ListCallCount()).To(Equal(1))
			Expect(tasksResponse).To(HaveLen(1))
			Expect(tasksResponse[0].GUID).To(Equal(taskGUID))
		})

		When("listing tasks fails", func() {
			BeforeEach(func() {
				taskClient.ListReturns(nil, errors.New("list-tasks-error"))
			})

			It("fails", func() {
				Expect(err).To(MatchError(ContainSubstring("list-tasks-error")))
			})
		})

		When("there are no tasks", func() {
			BeforeEach(func() {
				taskClient.ListReturns([]*opi.Task{}, nil)
			})

			It("fails", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(tasksResponse).NotTo(BeNil())
				Expect(tasksResponse).To(HaveLen(0))
			})
		})
	})

	Describe("Cancel Task", func() {
		BeforeEach(func() {
			taskClient.DeleteReturns("the/callback/url", nil)
		})

		JustBeforeEach(func() {
			err = taskBifrost.CancelTask(taskGUID)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the task", func() {
			Expect(taskClient.DeleteCallCount()).To(Equal(1))
			Expect(taskClient.DeleteArgsForCall(0)).To(Equal(taskGUID))
		})

		When("deleting the task fails", func() {
			BeforeEach(func() {
				taskClient.DeleteReturns("", errors.New("delete-task-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("delete-task-err")))
			})
		})

		It("notifies the cloud controller", func() {
			Eventually(jsonClient.PostCallCount).Should(Equal(1))

			url, data := jsonClient.PostArgsForCall(0)
			Expect(url).To(Equal("the/callback/url"))
			Expect(data).To(Equal(cf.TaskCompletedRequest{
				TaskGUID:      taskGUID,
				Failed:        true,
				FailureReason: "task was cancelled",
			}))
		})

		When("notifying the cloud controller fails", func() {
			BeforeEach(func() {
				jsonClient.PostReturns(errors.New("cc-error"))
			})

			It("still succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the callback URL is empty", func() {
			BeforeEach(func() {
				taskClient.DeleteReturns("", nil)
			})

			It("still succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not notify the cloud controller", func() {
				Consistently(jsonClient.PostCallCount).Should(BeZero())
			})
		})

		When("cloud controller notification takes forever", func() {
			It("still succeeds", func(done Done) {
				jsonClient.PostStub = func(string, interface{}) error {
					<-make(chan interface{}) // block forever

					return nil
				}

				err = taskBifrost.CancelTask(taskGUID)

				Expect(err).NotTo(HaveOccurred())

				close(done)
			})
		})
	})
})
