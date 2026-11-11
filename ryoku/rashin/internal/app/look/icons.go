package look

import (
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
)

var Icon struct {
	Search, Add, More, Close, Back, Forward, ChevronRight            *widget.Icon
	ExpandMore, ExpandLess, Settings, Ask, Agents, Projects, Index   *widget.Icon
	SidebarToggle, Fullscreen, FullscreenExit, Send, Stop, Attach    *widget.Icon
	Copy, Retry, Delete, Rename, OpenInNew, Terminal, Folder, Branch *widget.Icon
	Diff, Graph, Tasks, Activity, Lock, Unlock, Check                *widget.Icon
	Error, Warning, Fork, Undo, ArrowDown, Key, Link, Play, Filter   *widget.Icon
	Thought, Steps, File, Remove, FolderOpen, Show, Hide             *widget.Icon
	Quote, Goal, Pause, Screen                                       *widget.Icon
	Teams, Inbox, Routine, Vault, Report, Approval, Member, Mail     *widget.Icon
	Schedule, Spend, Import, Export, Lead                            *widget.Icon
}

// the filled variants mark the destination a navigation row stands for when it is the current one
var IconFill struct {
	Inbox, Teams, Tasks, Routine, Vault, Activity *widget.Icon
	Settings, Ask, Agents, Projects, Index        *widget.Icon
}

func init() {
	Icon.Search = kit.Icon(symbols.Search)
	Icon.Add = kit.Icon(symbols.Add)
	Icon.More = kit.Icon(symbols.MoreVert)
	Icon.Close = kit.Icon(symbols.Close)
	Icon.Back = kit.Icon(symbols.ArrowBack)
	Icon.Forward = kit.Icon(symbols.ArrowForward)
	Icon.ChevronRight = kit.Icon(symbols.ChevronRight)
	Icon.ExpandMore = kit.Icon(symbols.ExpandMore)
	Icon.ExpandLess = kit.Icon(symbols.ExpandLess)
	Icon.Settings = kit.Icon(symbols.Settings)
	Icon.Ask = kit.Icon(symbols.Forum)
	Icon.Agents = kit.Icon(symbols.SmartToy)
	Icon.Projects = kit.Icon(symbols.Folder)
	Icon.Index = kit.Icon(symbols.FindInPage)
	Icon.SidebarToggle = kit.Icon(symbols.SideNavigation)
	Icon.Fullscreen = kit.Icon(symbols.Fullscreen)
	Icon.FullscreenExit = kit.Icon(symbols.FullscreenExit)
	Icon.Send = kit.Icon(symbols.ArrowUpward)
	Icon.Stop = kit.Icon(symbols.Stop)
	Icon.Attach = kit.Icon(symbols.Attachment)
	Icon.Copy = kit.Icon(symbols.ContentCopy)
	Icon.Retry = kit.Icon(symbols.Refresh)
	Icon.Delete = kit.Icon(symbols.Delete)
	Icon.Rename = kit.Icon(symbols.Edit)
	Icon.OpenInNew = kit.Icon(symbols.OpenInNew)
	Icon.Terminal = kit.Icon(symbols.Terminal)
	Icon.Folder = kit.Icon(symbols.Folder)
	Icon.Branch = kit.Icon(symbols.ForkRight)
	Icon.Diff = kit.Icon(symbols.Difference)
	Icon.Graph = kit.Icon(symbols.DeviceHub)
	Icon.Tasks = kit.Icon(symbols.Assignment)
	Icon.Activity = kit.Icon(symbols.Timeline)
	Icon.Lock = kit.Icon(symbols.Lock)
	Icon.Unlock = kit.Icon(symbols.LockOpen)
	Icon.Check = kit.Icon(symbols.Check)
	Icon.Error = kit.Icon(symbols.Error)
	Icon.Warning = kit.Icon(symbols.Warning)
	Icon.Fork = kit.Icon(symbols.CallSplit)
	Icon.Undo = kit.Icon(symbols.Undo)
	Icon.ArrowDown = kit.Icon(symbols.ArrowDownward)
	Icon.Key = kit.Icon(symbols.Key)
	Icon.Link = kit.Icon(symbols.Link)
	Icon.Play = kit.Icon(symbols.PlayArrow)
	Icon.Filter = kit.Icon(symbols.FilterList)
	Icon.Thought = kit.Icon(symbols.Lightbulb)
	Icon.Steps = kit.Icon(symbols.FormatListBulleted)
	Icon.File = kit.Icon(symbols.Description)
	Icon.Remove = kit.Icon(symbols.Remove)
	Icon.FolderOpen = kit.Icon(symbols.FolderOpen)
	Icon.Show = kit.Icon(symbols.Visibility)
	Icon.Hide = kit.Icon(symbols.VisibilityOff)
	Icon.Quote = kit.Icon(symbols.FormatQuote)
	Icon.Goal = kit.Icon(symbols.MyLocation)
	Icon.Pause = kit.Icon(symbols.Pause)
	Icon.Screen = kit.Icon(symbols.DesktopWindows)
	Icon.Teams = kit.Icon(symbols.Group)
	Icon.Inbox = kit.Icon(symbols.Inbox)
	Icon.Routine = kit.Icon(symbols.Update)
	Icon.Vault = kit.Icon(symbols.VpnKey)
	Icon.Report = kit.Icon(symbols.AssignmentTurnedIn)
	Icon.Approval = kit.Icon(symbols.VerifiedUser)
	Icon.Member = kit.Icon(symbols.Person)
	Icon.Mail = kit.Icon(symbols.Mail)
	Icon.Schedule = kit.Icon(symbols.Schedule)
	Icon.Spend = kit.Icon(symbols.DataUsage)
	Icon.Import = kit.Icon(symbols.Download)
	Icon.Export = kit.Icon(symbols.Upload)
	Icon.Lead = kit.Icon(symbols.Stars)
	IconFill.Inbox = kit.Icon(symbols.InboxFill)
	IconFill.Teams = kit.Icon(symbols.GroupFill)
	IconFill.Tasks = kit.Icon(symbols.AssignmentFill)
	IconFill.Routine = kit.Icon(symbols.UpdateFill)
	IconFill.Vault = kit.Icon(symbols.VpnKeyFill)
	IconFill.Activity = kit.Icon(symbols.TimelineFill)
	IconFill.Settings = kit.Icon(symbols.SettingsFill)
	IconFill.Ask = kit.Icon(symbols.ForumFill)
	IconFill.Agents = kit.Icon(symbols.SmartToyFill)
	IconFill.Projects = kit.Icon(symbols.FolderFill)
	IconFill.Index = kit.Icon(symbols.FindInPageFill)
}
