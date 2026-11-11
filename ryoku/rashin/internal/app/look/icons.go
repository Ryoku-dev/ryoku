package look

import (
	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

var Icon struct {
	Search, Add, More, Close, Back, Forward, ChevronRight            *widget.Icon
	ExpandMore, ExpandLess, Settings, Ask, Agents, Projects, Index   *widget.Icon
	SidebarToggle, Fullscreen, FullscreenExit, Send, Stop, Attach    *widget.Icon
	Copy, Retry, Delete, Rename, OpenInNew, Terminal, Folder, Branch *widget.Icon
	Diff, Graph, Tasks, Activity, Lock, Unlock, Check                *widget.Icon
	Error, Warning, Fork, Undo, ArrowDown, Key, Link, Play, Filter   *widget.Icon
	Thought, Steps, File, Remove, FolderOpen, Blank, Show, Hide      *widget.Icon
	Quote, Goal, Pause, Screen                                       *widget.Icon
	Teams, Inbox, Routine, Vault, Report, Approval, Member, Mail     *widget.Icon
	Schedule, Spend, Import, Export, Lead                            *widget.Icon
}

func init() {
	Icon.Search = kit.Icon(mdicons.ActionSearch)
	Icon.Add = kit.Icon(mdicons.ContentAdd)
	Icon.More = kit.Icon(mdicons.NavigationMoreVert)
	Icon.Close = kit.Icon(mdicons.NavigationClose)
	Icon.Back = kit.Icon(mdicons.NavigationArrowBack)
	Icon.Forward = kit.Icon(mdicons.NavigationArrowForward)
	Icon.ChevronRight = kit.Icon(mdicons.NavigationChevronRight)
	Icon.ExpandMore = kit.Icon(mdicons.NavigationExpandMore)
	Icon.ExpandLess = kit.Icon(mdicons.NavigationExpandLess)
	Icon.Settings = kit.Icon(mdicons.ActionSettings)
	Icon.Ask = kit.Icon(mdicons.ActionQuestionAnswer)
	Icon.Agents = glyphAgent
	Icon.Projects = kit.Icon(mdicons.FileFolder)
	Icon.Index = kit.Icon(mdicons.ActionFindInPage)
	Icon.SidebarToggle = glyphSidebar
	Icon.Fullscreen = kit.Icon(mdicons.NavigationFullscreen)
	Icon.FullscreenExit = kit.Icon(mdicons.NavigationFullscreenExit)
	Icon.Send = kit.Icon(mdicons.NavigationArrowUpward)
	Icon.Stop = kit.Icon(mdicons.AVStop)
	Icon.Attach = kit.Icon(mdicons.FileAttachment)
	Icon.Copy = kit.Icon(mdicons.ContentContentCopy)
	Icon.Retry = kit.Icon(mdicons.NavigationRefresh)
	Icon.Delete = kit.Icon(mdicons.ActionDelete)
	Icon.Rename = kit.Icon(mdicons.EditorModeEdit)
	Icon.OpenInNew = kit.Icon(mdicons.ActionOpenInNew)
	Icon.Terminal = glyphTerminal
	Icon.Folder = kit.Icon(mdicons.FileFolder)
	Icon.Branch = glyphBranch
	Icon.Diff = glyphDiff
	Icon.Graph = kit.Icon(mdicons.HardwareDeviceHub)
	Icon.Tasks = kit.Icon(mdicons.ActionAssignment)
	Icon.Activity = kit.Icon(mdicons.ActionTimeline)
	Icon.Lock = kit.Icon(mdicons.ActionLock)
	Icon.Unlock = kit.Icon(mdicons.ActionLockOpen)
	Icon.Check = kit.Icon(mdicons.ActionDone)
	Icon.Error = kit.Icon(mdicons.AlertError)
	Icon.Warning = kit.Icon(mdicons.AlertWarning)
	Icon.Fork = kit.Icon(mdicons.CommunicationCallSplit)
	Icon.Undo = kit.Icon(mdicons.ContentUndo)
	Icon.ArrowDown = kit.Icon(mdicons.NavigationArrowDownward)
	Icon.Key = kit.Icon(mdicons.ActionLockOutline)
	Icon.Link = kit.Icon(mdicons.ContentLink)
	Icon.Play = kit.Icon(mdicons.AVPlayArrow)
	Icon.Filter = kit.Icon(mdicons.ContentFilterList)
	Icon.Thought = kit.Icon(mdicons.ActionLightbulbOutline)
	Icon.Steps = kit.Icon(mdicons.EditorFormatListBulleted)
	Icon.File = kit.Icon(mdicons.ActionDescription)
	Icon.Remove = kit.Icon(mdicons.ContentRemove)
	Icon.FolderOpen = kit.Icon(mdicons.FileFolderOpen)
	Icon.Blank = glyph(box(12, 12, 12, 12))
	Icon.Show = kit.Icon(mdicons.ActionVisibility)
	Icon.Hide = kit.Icon(mdicons.ActionVisibilityOff)
	Icon.Quote = kit.Icon(mdicons.EditorFormatQuote)
	Icon.Goal = kit.Icon(mdicons.DeviceGPSFixed)
	Icon.Pause = kit.Icon(mdicons.AVPause)
	Icon.Screen = kit.Icon(mdicons.HardwareDesktopWindows)
	Icon.Teams = kit.Icon(mdicons.SocialGroup)
	Icon.Inbox = kit.Icon(mdicons.ContentInbox)
	Icon.Routine = kit.Icon(mdicons.ActionUpdate)
	Icon.Vault = kit.Icon(mdicons.CommunicationVPNKey)
	Icon.Report = kit.Icon(mdicons.ActionAssignmentTurnedIn)
	Icon.Approval = kit.Icon(mdicons.ActionVerifiedUser)
	Icon.Member = kit.Icon(mdicons.SocialPerson)
	Icon.Mail = kit.Icon(mdicons.CommunicationEmail)
	Icon.Schedule = kit.Icon(mdicons.ActionSchedule)
	Icon.Spend = kit.Icon(mdicons.DeviceDataUsage)
	Icon.Import = kit.Icon(mdicons.FileFileDownload)
	Icon.Export = kit.Icon(mdicons.FileFileUpload)
	Icon.Lead = kit.Icon(mdicons.ActionStars)
}
