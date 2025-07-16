package ui

import (
	"fmt"
	"strconv"

	"github.com/askovpen/gossiped/pkg/config"
	"github.com/askovpen/gossiped/pkg/msgapi"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// AreaListQuit exit app
func (a *App) AreaListQuit() (string, tview.Primitive, bool, bool) {
	modal := NewModalMenu().
		SetText("Quit GOssipEd?").
		AddButtons([]string{
			"    Quit   ",
			"   Cancel  ",
		}).
		SetDoneFunc(func(buttonIndex int) {
			if buttonIndex == 0 {
				a.App.Stop()
			} else {
				a.Pages.HidePage("AreaListQuit")
				a.App.SetFocus(a.al)
				//a.Pages.SwitchToPage("AreaList")
			}
		})
	return "AreaListQuit", modal, false, false
}

func initAreaListHeader(a *App) {
	borderStyle := config.GetElementStyle(config.ColorAreaAreaList, config.ColorElementBorder)
	headerStyle := config.GetElementStyle(config.ColorAreaAreaList, config.ColorElementHeader)
	fgHeader, bgHeader, attrHeader := headerStyle.Decompose()
	selStyle := config.GetElementStyle(config.ColorAreaAreaList, config.ColorElementSelection)
	a.al.SetBorder(true).
		SetBorderStyle(borderStyle)
	a.al.SetSelectedStyle(selStyle)
	a.al.SetCell(
		0, 0, tview.NewTableCell(" Area").
			SetTextColor(fgHeader).SetBackgroundColor(bgHeader).SetAttributes(attrHeader).
			SetSelectable(false))
	a.al.SetCell(
		0, 1, tview.NewTableCell("EchoID").
			SetTextColor(fgHeader).SetBackgroundColor(bgHeader).SetAttributes(attrHeader).
			SetExpansion(1).
			SetSelectable(false))
	a.al.SetCell(
		0, 2, tview.NewTableCell("Msgs").
			SetTextColor(fgHeader).SetBackgroundColor(bgHeader).SetAttributes(attrHeader).
			SetSelectable(false).
			SetAlign(tview.AlignRight))
	a.al.SetCell(
		0, 3, tview.NewTableCell("   New").
			SetTextColor(fgHeader).SetBackgroundColor(bgHeader).SetAttributes(attrHeader).
			SetSelectable(false).
			SetAlign(tview.AlignRight))
}

func (a *App) RefreshAreaList() {
	var currentArea = ""
	if a.CurrentArea != nil {
		currentArea = (*a.CurrentArea).GetName()
	}
	refreshAreaList(a, currentArea)
}

func refreshAreaList(a *App, currentArea string) {
	refreshAreaListWithFilter(a, currentArea, "")
}

// getAreasForSelection returns the appropriate area slice based on search text
func getAreasForSelection(searchText string) []msgapi.FilteredArea {
	return msgapi.FilterAreas(searchText)
}

func refreshAreaListWithFilter(a *App, currentArea string, searchText string) {
	msgapi.SortAreas()
	a.al.Clear()
	initAreaListHeader(a)
	styleItem := config.GetElementStyle(config.ColorAreaAreaList, config.ColorElementItem)
	styleHighligt := config.GetElementStyle(config.ColorAreaAreaList, config.ColorElementHighlight)
	fgItem, bgItem, attrItem := styleItem.Decompose()
	fgHigh, bgHigh, attrHigh := styleHighligt.Decompose()
	var selectIndex = -1
	
	// Get filtered areas based on search text
	filteredAreas := msgapi.FilterAreas(searchText)
	
	for i, filtered := range filteredAreas {
		ar := filtered.AreaPrimitive
		fg, bg, attr := fgItem, bgItem, attrItem
		areaStyle := ""
		if msgapi.AreaHasUnreadMessages(&ar) {
			areaStyle = "+"
			fg, bg, attr = fgHigh, bgHigh, attrHigh
		}
		
		a.al.SetCell(i+1, 0, tview.NewTableCell(areaStyle+strconv.FormatInt(int64(filtered.OriginalIndex), 10)).
			SetAlign(tview.AlignRight).
			SetTextColor(fg).SetBackgroundColor(bg).SetAttributes(attr))
		a.al.SetCell(i+1, 1, tview.NewTableCell(ar.GetName()).
			SetTextColor(fg).SetBackgroundColor(bg).SetAttributes(attr))
		a.al.SetCell(i+1, 2, tview.NewTableCell(strconv.FormatInt(int64(ar.GetCount()), 10)).
			SetTextColor(fg).SetBackgroundColor(bg).SetAttributes(attr).
			SetAlign(tview.AlignRight))
		a.al.SetCell(i+1, 3, tview.NewTableCell(strconv.FormatInt(int64(ar.GetCount()-ar.GetLast()), 10)).
			SetTextColor(fg).SetBackgroundColor(bg).SetAttributes(attr).
			SetAlign(tview.AlignRight))
		if currentArea != "" && currentArea == ar.GetName() {
			selectIndex = i + 1
		}
	}
	
	// Auto-select first item if searching and no current area selected
	if searchText != "" && selectIndex == -1 && len(filteredAreas) > 0 {
		selectIndex = 1
	}
	
	if selectIndex != -1 {
		a.al.Select(selectIndex, 0)
	}
}

// AreaList - arealist widget
func (a *App) AreaList() (string, tview.Primitive, bool, bool) {
	searchString := NewSearchString()
	var currentSearchText string
	var disableSetSelectedFunc bool
	
	a.al = tview.NewTable().
		SetFixed(1, 0).
		SetSelectable(true, false).
		SetSelectionChangedFunc(func(row int, column int) {
			if row < 1 {
				row = 1
			}
			areas := getAreasForSelection(currentSearchText)
			
			if row-1 < len(areas) {
				var area = areas[row-1].AreaPrimitive
				a.sb.SetStatus(fmt.Sprintf("%s: %d msgs, %d unread",
					area.GetName(),
					area.GetCount(),
					area.GetCount()-area.GetLast(),
				))
			}
		})
	_, defBg, _ := config.StyleDefault.Decompose()
	a.al.SetBackgroundColor(defBg)
	a.al.SetSelectedFunc(func(row int, column int) {
		// This is called when double-clicking or other selection events
		if disableSetSelectedFunc {
			return
		}
		if currentSearchText == "" {
			a.onSelected(row, column)
		}
	})
	a.al.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch key := event.Key(); key {
		case tcell.KeyEsc:
			searchString.Clear()
			currentSearchText = ""
			disableSetSelectedFunc = false // Re-enable when returning to area list
			refreshAreaList(a, "")
			a.Pages.ShowPage("AreaListQuit")
		case tcell.KeyF1:
			a.Pages.ShowPage("AreaListHelp")
		case tcell.KeyRight, tcell.KeyEnter:
			// Disable SetSelectedFunc during our manual selection
			disableSetSelectedFunc = true
			
			row, _ := a.al.GetSelection()
			areas := getAreasForSelection(currentSearchText)
			
			// Do the selection with current state
			if row-1 < len(areas) {
				filtered := areas[row-1]
				a.CurrentArea = &msgapi.Areas[filtered.OriginalIndex]
			}
			
			if a.CurrentArea != nil {
				// Initialize area before first access
				(*a.CurrentArea).Init()
				lastMsg := (*a.CurrentArea).GetLast()
				countMsg := (*a.CurrentArea).GetCount()
				
				// Handle empty areas properly - allow access but use special message number
				var msgNum uint32
				if countMsg == 0 {
					msgNum = 1 // ViewMsg will handle this case properly
				} else if lastMsg == 0 && countMsg > 0 {
					msgNum = 1
				} else {
					msgNum = lastMsg
				}
				
				pageName := fmt.Sprintf("ViewMsg-%s-%d", (*a.CurrentArea).GetName(), msgNum)
				
				if a.Pages.HasPage(pageName) {
					a.Pages.SwitchToPage(pageName)
				} else {
					a.Pages.AddPage(a.ViewMsg(a.CurrentArea, msgNum))
					a.Pages.SwitchToPage(pageName)
				}
				
				// Clear search AFTER navigation
				searchString.Clear()
				currentSearchText = ""
			}
		case tcell.KeyDown, tcell.KeyUp:
			// Allow navigation within filtered list - don't clear search
			return event
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			searchString.RemoveChar()
			currentSearchText = searchString.GetText()
			refreshAreaListWithFilter(a, "", currentSearchText)
		case tcell.KeyRune:
			searchString.AddChar(event.Rune())
			currentSearchText = searchString.GetText()
			refreshAreaListWithFilter(a, "", currentSearchText)
		}
		return event
	})
	refreshAreaList(a, "")
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(searchString, 1, 1, false).
		AddItem(a.al, 0, 1, true)
	return "AreaList", layout, true, true
}
func (a *App) onSelected(row int, column int) {
	if row < 1 {
		row = 1
	}
	a.CurrentArea = &msgapi.Areas[row-1]
	if a.Pages.HasPage(fmt.Sprintf("ViewMsg-%s-%d", (*a.CurrentArea).GetName(), (*a.CurrentArea).GetLast())) {
		a.Pages.SwitchToPage(fmt.Sprintf("ViewMsg-%s-%d", (*a.CurrentArea).GetName(), (*a.CurrentArea).GetLast()))
	} else {
		a.Pages.AddPage(a.ViewMsg(a.CurrentArea, (*a.CurrentArea).GetLast()))
		a.Pages.SwitchToPage(fmt.Sprintf("ViewMsg-%s-%d", (*a.CurrentArea).GetName(), (*a.CurrentArea).GetLast()))
	}
}

