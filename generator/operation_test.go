package generator_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/executor"
	"github.com/cloudfoundry-incubator/rep"
	"github.com/cloudfoundry-incubator/rep/generator"
	"github.com/cloudfoundry-incubator/rep/generator/internal"
	"github.com/cloudfoundry-incubator/rep/generator/internal/fake_internal"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Operation", func() {
	Describe("MissingLRPOperation", func() {
		var (
			containerDelegate   *fake_internal.FakeContainerDelegate
			missingLRPOperation *generator.MissingLRPOperation
			lrpKey              models.ActualLRPKey
			containerKey        models.ActualLRPContainerKey
		)

		BeforeEach(func() {
			lrpKey = models.NewActualLRPKey("the-process-guid", 0, "the-domain")
			containerKey = models.NewActualLRPContainerKey("the-instance-guid", "the-cell-id")
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			missingLRPOperation = generator.NewMissingLRPOperation(logger, fakeBBS, containerDelegate, lrpKey, containerKey)
		})

		Describe("Key", func() {
			It("returns the InstanceGuid", func() {
				Ω(missingLRPOperation.Key()).Should(Equal("the-instance-guid"))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-missing-lrp-operation"

			JustBeforeEach(func() {
				missingLRPOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Ω(containerDelegate.GetContainerCallCount()).Should(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Ω(containerGuid).Should(Equal(containerKey.InstanceGuid))
				Ω(containerDelegateLogger.SessionName()).Should(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Ω(logger).Should(Say(sessionName + ".starting"))
				Ω(logger).Should(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("removes the actualLRP", func() {
					Ω(fakeBBS.RemoveActualLRPCallCount()).Should(Equal(1))
					actualLRPKey, actualLRPContainerKey, bbsLogger := fakeBBS.RemoveActualLRPArgsForCall(0)
					Ω(actualLRPKey).Should(Equal(lrpKey))
					Ω(actualLRPContainerKey).Should(Equal(containerKey))
					Ω(bbsLogger.SessionName()).Should(Equal(sessionName))
				})
			})

			Context("when the container exists", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				It("does not remove the actualLRP", func() {
					Ω(fakeBBS.RemoveActualLRPCallCount()).Should(Equal(0))
				})

				It("logs that it skipped the operation because the container was found", func() {
					Ω(logger).Should(Say(sessionName + ".skipped-because-container-exists"))
				})
			})
		})
	})

	Describe("MissingTaskOperation", func() {
		var (
			containerDelegate    *fake_internal.FakeContainerDelegate
			missingTaskOperation *generator.MissingTaskOperation
			taskGuid             string
		)

		BeforeEach(func() {
			taskGuid = "the-task-guid"
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			missingTaskOperation = generator.NewMissingTaskOperation(logger, fakeBBS, containerDelegate, taskGuid)
		})

		Describe("Key", func() {
			It("returns the TaskGuid", func() {
				Ω(missingTaskOperation.Key()).Should(Equal("the-task-guid"))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-missing-task-operation"

			JustBeforeEach(func() {
				missingTaskOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Ω(containerDelegate.GetContainerCallCount()).Should(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Ω(containerGuid).Should(Equal("the-task-guid"))
				Ω(containerDelegateLogger.SessionName()).Should(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Ω(logger).Should(Say(sessionName + ".starting"))
				Ω(logger).Should(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("fetches the task", func() {
					Ω(fakeBBS.TaskByGuidCallCount()).Should(Equal(1))
					actualTaskGuid := fakeBBS.TaskByGuidArgsForCall(0)
					Ω(actualTaskGuid).To(Equal(taskGuid))
				})

				Context("when the task is Running", func() {
					BeforeEach(func() {
						fakeBBS.TaskByGuidReturns(models.Task{State: models.TaskStateRunning}, nil)
					})

					It("fails the task", func() {
						Ω(fakeBBS.FailTaskCallCount()).Should(Equal(1))
						actualLogger, actualTaskGuid, actualFailureReason := fakeBBS.FailTaskArgsForCall(0)
						Ω(actualLogger.SessionName()).Should(Equal(sessionName))
						Ω(actualTaskGuid).Should(Equal(taskGuid))
						Ω(actualFailureReason).Should(Equal(internal.TaskCompletionReasonMissingContainer))
					})

					Context("when failing the task fails", func() {
						BeforeEach(func() {
							fakeBBS.FailTaskReturns(errors.New("failed"))
						})

						It("logs the failure", func() {
							Ω(logger).Should(Say(sessionName + ".failed-to-fail-task"))
						})
					})
				})

				Context("when the task is not Running", func() {
					BeforeEach(func() {
						fakeBBS.TaskByGuidReturns(models.Task{State: models.TaskStateCompleted}, nil)
					})

					It("does not fail the task", func() {
						Ω(fakeBBS.FailTaskCallCount()).Should(Equal(0))
					})

					It("logs the fact that it skipped", func() {
						Ω(logger).Should(Say(sessionName + ".skipped-because-task-is-not-running"))
					})
				})

				Context("when fetching the task fails", func() {
					BeforeEach(func() {
						fakeBBS.TaskByGuidReturns(models.Task{}, errors.New("failed"))
					})

					It("logs the failure", func() {
						Ω(logger).Should(Say(sessionName + ".failed-to-fetch-task"))
					})
				})
			})

			Context("when the container exists", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				It("does not fail the task", func() {
					Ω(fakeBBS.FailTaskCallCount()).Should(Equal(0))
				})

				It("logs that it skipped the operation because the container was found", func() {
					Ω(logger).Should(Say(sessionName + ".skipped-because-container-exists"))
				})
			})
		})
	})

	Describe("ContainerOperation", func() {
		var (
			containerDelegate  *fake_internal.FakeContainerDelegate
			lrpProcessor       *fake_internal.FakeLRPProcessor
			taskProcessor      *fake_internal.FakeTaskProcessor
			containerOperation *generator.ContainerOperation
			guid               string
		)

		BeforeEach(func() {
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			lrpProcessor = new(fake_internal.FakeLRPProcessor)
			taskProcessor = new(fake_internal.FakeTaskProcessor)
			guid = "the-guid"
			containerOperation = generator.NewContainerOperation(logger, lrpProcessor, taskProcessor, containerDelegate, guid)
		})

		Describe("Key", func() {
			It("returns the Guid", func() {
				Ω(containerOperation.Key()).Should(Equal("the-guid"))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-container-operation"

			JustBeforeEach(func() {
				containerOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Ω(containerDelegate.GetContainerCallCount()).Should(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Ω(containerGuid).Should(Equal(guid))
				Ω(containerDelegateLogger.SessionName()).Should(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Ω(logger).Should(Say(sessionName + ".starting"))
				Ω(logger).Should(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("logs that it skipped the operation because the container was found", func() {
					Ω(logger).Should(Say(sessionName + ".skipped-because-container-does-not-exist"))
				})

				It("does not farm the container out to any processor", func() {
					Ω(lrpProcessor.ProcessCallCount()).Should(Equal(0))
					Ω(taskProcessor.ProcessCallCount()).Should(Equal(0))
				})
			})

			Context("when the container exists", func() {
				var (
					container executor.Container
				)

				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				Context("when the container has an LRP lifecycle tag", func() {
					BeforeEach(func() {
						container = executor.Container{
							Tags: executor.Tags{
								rep.LifecycleTag: rep.LRPLifecycle,
							},
						}
						containerDelegate.GetContainerReturns(container, true)
					})

					It("farms the container out to only the lrp processor", func() {
						Ω(lrpProcessor.ProcessCallCount()).Should(Equal(1))
						Ω(taskProcessor.ProcessCallCount()).Should(Equal(0))
						actualLogger, actualContainer := lrpProcessor.ProcessArgsForCall(0)
						Ω(actualLogger.SessionName()).Should(Equal(sessionName))
						Ω(actualContainer).Should(Equal(container))
					})
				})

				Context("when the container has a Task lifecycle tag", func() {
					BeforeEach(func() {
						container = executor.Container{
							Tags: executor.Tags{
								rep.LifecycleTag: rep.TaskLifecycle,
							},
						}
						containerDelegate.GetContainerReturns(container, true)
					})

					It("farms the container out to only the task processor", func() {
						Ω(taskProcessor.ProcessCallCount()).Should(Equal(1))
						Ω(lrpProcessor.ProcessCallCount()).Should(Equal(0))
						actualLogger, actualContainer := taskProcessor.ProcessArgsForCall(0)
						Ω(actualLogger.SessionName()).Should(Equal(sessionName))
						Ω(actualContainer).Should(Equal(container))
					})
				})

				Context("when the container has an unknown lifecycle tag", func() {
					BeforeEach(func() {
						container = executor.Container{
							Tags: executor.Tags{
								rep.LifecycleTag: "some-other-tag",
							},
						}
						containerDelegate.GetContainerReturns(container, true)
					})

					It("does not farm the container out to any processor", func() {
						Ω(lrpProcessor.ProcessCallCount()).Should(Equal(0))
						Ω(taskProcessor.ProcessCallCount()).Should(Equal(0))
					})

					It("logs the unknown lifecycle", func() {
						Ω(logger).Should(Say(sessionName + ".failed-to-process-container-with-unknown-lifecycle"))
					})
				})
			})
		})
	})
})