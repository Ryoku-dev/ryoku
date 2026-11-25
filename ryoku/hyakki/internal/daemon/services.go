package daemon

import (
	"context"
	"encoding/json"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// servicesBusy reports work the daemon must stay up for although no client
// is connected: a model download, a llama-server request or start, or a
// decision consolidation.
func (d *Daemon) servicesBusy() bool {
	if d.deps.Local != nil && d.deps.Local.Busy() {
		return true
	}
	if d.deps.Decisions != nil && d.deps.Decisions.Busy() {
		return true
	}
	return d.deps.LearningBusy != nil && d.deps.LearningBusy()
}

// closeServices ends what the services hold at shutdown: the llama-server
// child, the decision queue and the grievance store.
func (d *Daemon) closeServices() {
	if d.deps.Local != nil {
		if err := d.deps.Local.Close(); err != nil {
			d.warn("close local models: %v", err)
		}
	}
	if d.deps.Decisions != nil {
		if err := d.deps.Decisions.Close(); err != nil {
			d.warn("close decisions: %v", err)
		}
	}
	if d.deps.Grievances != nil {
		if err := d.deps.Grievances.Close(); err != nil {
			d.warn("close grievances: %v", err)
		}
	}
}

// localResult is Rashin's own models as local.models and the snapshot show
// them; empty without the local models service.
func (d *Daemon) localResult(ctx context.Context) protocol.LocalResult {
	if d.deps.Local == nil {
		return protocol.LocalResult{}
	}
	return d.deps.Local.Models(ctx)
}

// localCall serves one local.* method. Reading the models and searching
// Hugging Face are open to every client; what downloads, deletes or starts
// something is the Rashin app's alone. Progress and the changes a download
// makes later arrive from the service itself as local.progress and
// local.changed.
func (d *Daemon) localCall(ctx context.Context, c *connection, method string, raw json.RawMessage) (any, error) {
	if d.deps.Local == nil {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hyakki's own models are not available."))
	}
	switch method {
	case protocol.MLocalModels:
		return d.deps.Local.Models(ctx), nil
	case protocol.MLocalSearch:
		p, err := unmarshal[protocol.LocalSearchParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Local.Search(ctx, p.Repo)
	}
	if err := requireApp(c); err != nil {
		return nil, err
	}
	switch method {
	case protocol.MLocalFound:
		if _, err := unmarshal[protocol.Empty](raw); err != nil {
			return nil, err
		}
		models, err := d.deps.Local.Found(ctx, d.Settings().Providers.Models.Folders)
		return protocol.LocalFoundResult{Models: models}, err
	case protocol.MLocalUseFound:
		p, err := unmarshal[protocol.LocalUseFoundParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Local.UseFound(p.Path)
	case protocol.MLocalDownload:
		p, err := unmarshal[protocol.LocalModelParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.deps.Local.Download(ctx, p.ID)
	case protocol.MLocalCancel:
		p, err := unmarshal[protocol.LocalModelParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.deps.Local.Cancel(p.ID)
	case protocol.MLocalRemove:
		p, err := unmarshal[protocol.LocalModelParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Local.Remove(ctx, p.ID)
	case protocol.MLocalAdd:
		p, err := unmarshal[protocol.LocalAddParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Local.Add(ctx, p)
	case protocol.MLocalRuntimeInstall:
		p, err := unmarshal[protocol.LocalRuntimeParams](raw)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, d.deps.Local.InstallRuntime(ctx, p.Backend)
	case protocol.MLocalStart:
		p, err := unmarshal[protocol.LocalModelParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Local.Start(ctx, p.ID)
	case protocol.MLocalStop:
		if _, err := unmarshal[protocol.LocalModelParams](raw); err != nil {
			return nil, err
		}
		d.deps.Local.Stop()
		return d.deps.Local.Models(ctx).Runtime, nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Method %s is not supported.", method))
}

// learningCall serves one learning.* method. Reading is open to every
// client; deleting, saving and forging are the Rashin app's alone. Both work
// with the feature on or off, so what a feature kept stays visible and
// removable after it is turned off, except forging a rule, which the rules
// service refuses while rules are off. A change goes out as
// learning.changed.
func (d *Daemon) learningCall(ctx context.Context, c *connection, method string, raw json.RawMessage) (any, error) {
	switch method {
	case protocol.MLearningSkillDelete, protocol.MLearningDecisionDelete, protocol.MLearningRuleSave,
		protocol.MLearningRuleDelete, protocol.MLearningRuleForge, protocol.MLearningGrievancesClear:
		if err := requireApp(c); err != nil {
			return nil, err
		}
	}
	unavailable := protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("This part of learning is not available."))
	changed := func(kind, project string) {
		d.Broadcast(protocol.EvtLearningChanged, protocol.LearningChanged{Kind: kind, Project: project})
	}
	switch method {
	case protocol.MLearningSkills:
		if d.deps.ManagedSkills == nil {
			return nil, unavailable
		}
		return d.deps.ManagedSkills.List(ctx)
	case protocol.MLearningSkillDelete:
		if d.deps.ManagedSkills == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.LearningSkillParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.ManagedSkills.Delete(ctx, p.Name)
		if err == nil {
			changed(protocol.LearningSkills, "")
		}
		return out, err
	case protocol.MLearningDecisions:
		if d.deps.Decisions == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.LearningDecisionsParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Decisions.Banks(ctx, p.Project)
	case protocol.MLearningDecisionDelete:
		if d.deps.Decisions == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.DecisionDeleteParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.Decisions.Delete(ctx, p)
		if err == nil {
			changed(protocol.LearningDecisions, p.Project)
		}
		return out, err
	case protocol.MLearningRules:
		if d.deps.Rules == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.LearningRulesParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Rules.List(ctx, p.Project)
	case protocol.MLearningRuleSave:
		if d.deps.Rules == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.RuleSaveParams](raw)
		if err != nil {
			return nil, err
		}
		out, err := d.deps.Rules.Save(ctx, p)
		if err == nil {
			changed(protocol.LearningRules, out.Project)
		}
		return out, err
	case protocol.MLearningRuleDelete:
		if d.deps.Rules == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.RuleRef](raw)
		if err != nil {
			return nil, err
		}
		if err := d.deps.Rules.Delete(ctx, p); err != nil {
			return nil, err
		}
		changed(protocol.LearningRules, p.Project)
		return protocol.Empty{}, nil
	case protocol.MLearningRuleForge:
		if d.deps.Rules == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.RuleForgeParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Rules.Forge(ctx, p)
	case protocol.MLearningGrievances:
		if d.deps.Grievances == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.LearningGrievancesParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Grievances.List(ctx, p)
	case protocol.MLearningGrievancesClear:
		if d.deps.Grievances == nil {
			return nil, unavailable
		}
		p, err := unmarshal[protocol.GrievancesClearParams](raw)
		if err != nil {
			return nil, err
		}
		if err := d.deps.Grievances.Clear(ctx, p); err != nil {
			return nil, err
		}
		changed(protocol.LearningGrievances, "")
		return protocol.Empty{}, nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Method %s is not supported.", method))
}
