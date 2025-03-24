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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Step struct {
	Name        string
	Description string
	Action      func() StepResult
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
func showMessageModal(app *tview.Application, message string, flex *tview.Flex) {
	responseChan := make(chan bool, 1)
	timeoutChan := make(chan bool, 1)

	go func() {
		time.Sleep(3 * time.Second)
		select {
		case timeoutChan <- true:
		default:
		}
	}()

	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			select {
			case responseChan <- true:
			default:
			}
		})

	app.QueueUpdateDraw(func() {
		app.SetRoot(modal, true)
	})

	select {
	case <-responseChan:
	case <-timeoutChan:
	}

	app.QueueUpdateDraw(func() {
		app.SetRoot(flex, true)
	})
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
			result := step.Action()
			duration := time.Since(startTime)

			if result.Error != nil {
				finalErr = result.Error
				logWriter("[red]Error: %v[white]", result.Error)
				shouldBreak = true
			} else {
				if result.Message != "" {
					logWriter("[blue]Message: %s[white]", result.Message)
					showMessageModal(app, result.Message, flex)
				}
				logWriter("[green]Completed in %v[white]", duration.Round(time.Millisecond))
				app.QueueUpdateDraw(func() {
					list.SetItemText(i, fmt.Sprintf("[green]✓ %s[white]", step.Name), step.Description)
				})
			}

			if shouldBreak {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}

		app.QueueUpdateDraw(func() {
			statusBar.Clear()
			if finalErr != nil {
				fmt.Fprintf(statusBar, "[red]Execution failed: %v[white]", finalErr)
			} else {
				fmt.Fprintf(statusBar, "[green]All steps completed successfully![white]")
			}
		})

		app.QueueUpdateDraw(func() {
			modalText := "Execution completed."
			if finalErr != nil {
				modalText = fmt.Sprintf("Execution failed: %v", finalErr)
			}
			modal := tview.NewModal().
				SetText(modalText).
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
		app.Stop()
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
				} else if strings.Contains(mainText, "→") {
					status = "[→]"
				}
			}
		}
		fmt.Printf("%s %s\n", status, step.Name)
	}

	fmt.Println()
	if viper.GetBool("FIRST_NODE") {
		fmt.Println("To setup additional nodes to join the cluster, run the command in additional_node_command.txt")
	} else {
		fmt.Println("The content of longhorn_drive_setup.txt must be run in order to mount drives properly. " +
			"This can be done in the control node, which was installed first, or with a valid kubeconfig for the cluster.")
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

type StepResult struct {
	Error   error
	Message string
}
type OptionResult struct {
	Selected []string
	Indexes  []int
	Canceled bool
}

func ShowOptionsScreen(title string, message string, options []string, preSelected []string) (OptionResult, error) {
	if globalApp == nil {
		return OptionResult{Canceled: true}, errors.New("application not initialized")
	}
	result := make(chan OptionResult, 1)
	optionsList := tview.NewList()
	optionsList.SetBorder(true)
	optionsList.SetTitle(title)
	selected := make(map[int]bool)
	for i, opt := range options {
		for _, preSelect := range preSelected {
			if opt == preSelect {
				selected[i] = true
				break
			}
		}
	}

	toggleSelection := func(index int, option string) {
		if index+1 >= optionsList.GetItemCount() {
			return
		}
		if selected[index] {
			delete(selected, index)
			optionsList.SetItemText(index+1, fmt.Sprintf("[ ] %s", option), "")
		} else {
			selected[index] = true
			optionsList.SetItemText(index+1, fmt.Sprintf("[green]✓ %s[white]", option), "")
		}
	}

	optionsList.AddItem("[green]✓ Done[white]", "Confirm selections", 'd', func() {
		var selectedItems []string
		var selectedIndexes []int
		for idx := range selected {
			selectedItems = append(selectedItems, options[idx])
			selectedIndexes = append(selectedIndexes, idx)
		}
		result <- OptionResult{
			Selected: selectedItems,
			Indexes:  selectedIndexes,
			Canceled: false,
		}
	})

	for i, option := range options {
		index := i
		text := "[ ] %s"
		if selected[index] {
			text = "[green]✓ %s[white]"
		}
		optionsList.AddItem(fmt.Sprintf(text, option), "", rune('1'+i), func() {
			toggleSelection(index, option)
		})
	}

	optionsList.AddItem("[red]Cancel[white]", "Abort and close", 'q', func() {
		go func() {
			result <- OptionResult{Canceled: true}
			globalApp.Stop()
		}()
	})

	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	if message != "" {
		messageView := tview.NewTextView()
		messageView.SetText(message)
		messageView.SetWordWrap(true)
		messageView.SetBorder(true)
		messageView.SetTitle("Message")
		messageView.SetDynamicColors(true)
		messageView.SetScrollable(true)
		messageView.SetChangedFunc(func() { globalApp.Draw() })

		lines := countLines(message)
		height := lines + 2
		flex.AddItem(messageView, height, 0, false)
		flex.AddItem(tview.NewBox(), 1, 0, false)

		currentRow := 0
		_, rows := messageView.GetScrollOffset()

		messageView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyUp:
				if currentRow > 0 {
					currentRow--
					messageView.ScrollTo(0, currentRow)
				}
				return nil
			case tcell.KeyDown:
				currentRow++
				messageView.ScrollTo(0, currentRow)
				return nil
			case tcell.KeyPgUp:
				currentRow = max(0, currentRow-rows)
				messageView.ScrollTo(0, currentRow)
				return nil
			case tcell.KeyPgDn:
				currentRow += rows
				messageView.ScrollTo(0, currentRow)
				return nil
			case tcell.KeyTab:
				globalApp.SetFocus(optionsList)
				return nil
			}
			return event
		})
	}

	optionsList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			currentIndex := optionsList.GetCurrentItem() - 1
			if currentIndex >= 0 && currentIndex < len(options) {
				toggleSelection(currentIndex, options[currentIndex])
			}
			return nil
		}
		return event
	})

	flex.AddItem(optionsList, 0, 3, true)

	globalApp.QueueUpdateDraw(func() {
		globalApp.SetRoot(flex, true)
		globalApp.SetFocus(optionsList)
	})

	selectedOption := <-result

	if selectedOption.Canceled {
		return selectedOption, errors.New("user canceled the selection")
	}

	return selectedOption, nil
}

func countLines(text string) int {
	lines := len(strings.Split(text, "\n"))
	if lines > 25 {
		return 25
	}
	return lines
}
