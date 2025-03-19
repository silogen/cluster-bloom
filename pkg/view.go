/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
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
 *  limitations under the License.
**/

package pkg

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"
)

type Step struct {
	Name        string
	Description string
	Action      func() error
}

var (
	globalApp         *tview.Application
	globalLogView     *tview.TextView
	ContinueOnFailure bool = false
)

func LogToUI(message string) {
	if globalApp != nil && globalLogView != nil {
		globalApp.QueueUpdateDraw(func() {
			fmt.Fprintln(globalLogView, message)
			globalLogView.ScrollToEnd()
		})
	}
}

func RunStepsWithUI(steps []Step) error {

	var logView *tview.TextView
	app := tview.NewApplication()
	list := tview.NewList()
	list.SetBorder(true).
		SetTitle("Installation Steps")
	list.SetSelectedBackgroundColor(tcell.ColorDefault).
		SetSelectedTextColor(tcell.ColorDefault)

	logView = tview.NewTextView()
	logView.SetDynamicColors(true).
		SetScrollable(true).
		ScrollToEnd().
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw()
		}).
		SetBorder(true).
		SetTitle("Logs")

	statusBar := tview.NewTextView()
	statusBar.SetDynamicColors(true).
		SetBorder(true).
		SetTitle("Status")

	for i, step := range steps {
		list.AddItem(fmt.Sprintf("[ ] %s", step.Name), step.Description, rune('1'+i), nil)
	}

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(logView, 0, 2, false).
		AddItem(statusBar, 3, 1, false)

	done := make(chan bool)
	var finalErr error

	go watchLogFile(app, logView)

	logWriter := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		app.QueueUpdateDraw(func() {
			fmt.Fprintf(logView, "%s\n", msg)
			log.Info(msg)
		})
	}
	globalApp = app
	globalLogView = logView
	go func() {
		var shouldBreak bool

		for i, step := range steps {
			app.QueueUpdateDraw(func() {
				statusBar.Clear()
				fmt.Fprintf(statusBar, "[yellow]Running: %s[white]", step.Name)
				list.SetItemText(i, fmt.Sprintf("[yellow]→ %s[white]", step.Name), step.Description)
			})

			logWriter("[blue]Starting step: %s[white]", step.Name)

			startTime := time.Now()
			err := step.Action()
			duration := time.Since(startTime)

			if err != nil {
				finalErr = err
				logWriter("[red]Error: %v[white]", err)
				app.QueueUpdateDraw(func() {
					statusBar.Clear()
					fmt.Fprintf(statusBar, "[red]Failed: %s[white]", step.Name)
					list.SetItemText(i, fmt.Sprintf("[red]✗ %s[white]", step.Name), step.Description)
				})

				responseChan := make(chan string)
				timeoutChan := make(chan bool, 1)

				go func() {
					time.Sleep(5 * time.Second)
					select {
					case timeoutChan <- true:
					default:
					}
				}()

				modal := tview.NewModal().
					SetText(fmt.Sprintf("Step failed: %s\nError: %v\n\n(Will automatically exit in 5 seconds)", step.Name, err)).
					AddButtons([]string{"Exit", "Continue"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						select {
						case responseChan <- buttonLabel:
						default:
						}
					})

				app.QueueUpdateDraw(func() {
					app.SetRoot(modal, true)
				})

				select {
				case buttonLabel := <-responseChan:
					if buttonLabel == "Continue" {
						logWriter("[yellow]Continuing despite error...[white]")
					} else {
						logWriter("[yellow]Exiting due to user request...[white]")
						done <- true
						return
					}
				case <-timeoutChan:
					logWriter("[yellow]No response received within timeout. Exiting...[white]")
					time.Sleep(500 * time.Millisecond)
					done <- true
					return
				}

				app.QueueUpdateDraw(func() {
					app.SetRoot(flex, true)
				})
			} else {
				logWriter("[green]Completed in %v[white]", duration.Round(time.Millisecond))
				app.QueueUpdateDraw(func() {
					list.SetItemText(i, fmt.Sprintf("[green]✓ %s[white]", step.Name), step.Description)
				})
			}

			time.Sleep(500 * time.Millisecond)
		}

		if !shouldBreak {
			app.QueueUpdateDraw(func() {
				statusBar.Clear()
				fmt.Fprintf(statusBar, "[green]All steps completed successfully![white]")
			})
			app.QueueUpdateDraw(func() {
				modal := tview.NewModal().
					SetText("All tasks completed successfully.").
					AddButtons([]string{"OK"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						app.SetRoot(flex, true)
						done <- true
					})
				pages := tview.NewPages().
					AddPage("background", flex, true, true).
					AddPage("modal", modal, true, true)
				app.SetRoot(pages, true)
			})
			return
		}
		done <- true
	}()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			done <- true
			return nil
		}
		return event
	})

	appErr := make(chan error, 1)
	go func() {
		appErr <- app.SetRoot(flex, true).Run()
	}()

	select {
	case <-done:
		app.Stop()
	case err := <-appErr:
		return err
	}

	fmt.Println("\n=== Cluster Bloom Execution Summary ===")
	fmt.Println()

	for i, step := range steps {
		status := "[ ]"
		if i < len(steps) {
			mainText, _ := list.GetItemText(i)
			if mainText != "" {
				if strings.Contains(mainText, "✓") {
					status = "[✓]"
				} else if strings.Contains(mainText, "✗") {
					status = "[✗]"
				}
			}
		}
		fmt.Printf("%s %s\n", status, step.Name)
	}

	fmt.Println()
	if finalErr != nil {
		fmt.Printf("Execution failed: %v\n", finalErr)
		return finalErr
	} else {
		fmt.Println("All steps completed successfully!")
	}

	return nil
}

func watchLogFile(app *tview.Application, logView *tview.TextView) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Could not determine current directory: %v\n", err)
		return
	}
	logPath := filepath.Join(currentDir, "bloom.log")
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			time.Sleep(500 * time.Millisecond)
		} else {
			break
		}
	}
	file, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("Could not open log file for reading: %v\n", err)
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		fmt.Printf("Could not stat log file: %v\n", err)
		return
	}
	currentSize := stat.Size()
	file.Seek(currentSize, 0)
	scanner := bufio.NewScanner(file)
	for {
		stat, err := file.Stat()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		if offset, err := file.Seek(0, 1); err == nil && stat.Size() < offset {
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
		}
		for scanner.Scan() {
			line := scanner.Text()
			app.QueueUpdateDraw(func() {
				fmt.Fprintln(logView, line)
				logView.ScrollToEnd()
			})
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func LogMessage(level LogLevel, message string) {
	switch level {
	case Debug:
		log.Debug(message)
	case Info:
		log.Info(message)
	case Warn:
		log.Warn(message)
	case Error:
		log.Error(message)
	default:
		log.Info(message)
	}
	LogToUI(message)
}

func LogCommand(commandName string, output string) {
	header := "Command output from " + commandName + ":"
	log.Info(header)
	LogToUI(header)
	log.Info(output)
	LogToUI(output)
}

type LogLevel int

const (
	Debug LogLevel = iota
	Info
	Warn
	Error
)
