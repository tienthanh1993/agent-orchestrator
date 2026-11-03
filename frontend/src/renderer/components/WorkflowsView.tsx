import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, GitBranch, Play, Plus, Save } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";
import { useWorkflows, useSaveWorkflow, useStartWorkflow, useWorkflowRuns, type Workflow, type WorkflowInput } from "../hooks/useWorkflows";
import { useWorkspaceQuery } from "../hooks/useWorkspaceQuery";
import { WorkflowStageEditor, WorkflowRunDetail, WorkflowSchedule } from "./WorkflowDetails";
import { WorkflowDefaults, WorkflowInputDialog, WorkflowRunInputs } from "./WorkflowInputs";
import { parseWorkflowInputs } from "../lib/workflow-inputs";
import { cn } from "../lib/utils";
import { CLOUD_PROJECT_KIND, STANDALONE_PROJECT_KIND } from "../types/workspace";
import "./workflows.css";

const initial = (): WorkflowInput => ({ name: "Research to implementation", projectId: "", task: "", enabled: false,
	timezone: Intl.DateTimeFormat().resolvedOptions().timeZone, rrule: "", stages: [
		{ name: "Research", agent: "codex", instructions: "Research the task and constraints. Save findings and sources in research.md.", approval: false },
		{ name: "Plan", agent: "codex", instructions: "Read the research artifacts. Write an actionable plan.md with acceptance criteria.", approval: true },
		{ name: "Build", agent: "codex", instructions: "Follow plan.md, implement the changes and verify them. Save test-results.md.", approval: false },
	] });

export function WorkflowsView() {
	const { t } = useTranslation();
	const query = useWorkflows();
	const save = useSaveWorkflow();
	const start = useStartWorkflow();
	const projects = (useWorkspaceQuery().data ?? []).filter(p => !p.hostId && p.kind !== CLOUD_PROJECT_KIND && p.kind !== STANDALONE_PROJECT_KIND);
	const [id, setId] = useState<string>();
	const [draft, setDraft] = useState<WorkflowInput>(initial);
	const [selected, setSelected] = useState(0);
	const [runId, setRunId] = useState<string>();
	const [dirty, setDirty] = useState(false);
	const [error, setError] = useState("");
	const [inputDialog, setInputDialog] = useState(false);
	const [submitting, setSubmitting] = useState(false);
	const pending = useRef(false);
	const runButton = useRef<HTMLButtonElement>(null);
	const inputs = parseWorkflowInputs(draft.task);
	const history = useWorkflowRuns(id);
	const run = history.data?.find(r => r.id === runId);
	const shown = run?.definition ?? draft;
	const stages = shown.stages;
	const busy = submitting || save.isPending || start.isPending;
	const update = (change: Partial<WorkflowInput>) => { setDraft(d => ({ ...d, ...change })); setDirty(true); };
	function open(w?: Workflow) {
		setId(w?.id); setDraft(w ? { ...w } : initial()); setSelected(0); setRunId(undefined); setDirty(false); setError("");
	}
	async function persist(andRun = false, values?: Record<string, string>) {
		if (pending.current) return;
		pending.current = true; setSubmitting(true);
		setError("");
		try {
			const w = await save.mutateAsync({ id, body: draft });
			setId(w.id); setDraft(w); setDirty(false);
			if (andRun) { const r = await start.mutateAsync({ id: w.id, inputs: values }); setRunId(r.id); setSelected(0); setInputDialog(false); }
		} catch (e) {
			if (inputDialog) throw e;
			setError(e instanceof Error ? e.message : "Unable to save workflow");
		} finally { pending.current = false; setSubmitting(false); }
	}
	function requestRun() {
		if (inputs.error) { setError(t(inputs.error)); return; }
		setError("");
		if (inputs.names.length) setInputDialog(true);
		else void persist(true);
	}
	return <div className="workflow-page">
		<header className="workflow-heading"><div><h1>{t("workflows.title")}</h1><p>{t("workflows.subtitle")}</p></div>
			<Button variant="outline" size="sm" disabled={busy} onClick={() => open()}><Plus />{t("workflows.new")}</Button></header>
		<div className="workflow-body">
			<aside className="workflow-library" aria-label={t("workflows.saved")}>
				<h2>{t("workflows.library")}</h2>
				{query.isLoading ? <p>{t("workflows.loading")}</p> : null}
				{query.error ? <p role="alert">{query.error.message}</p> : null}
				{query.data?.length === 0 ? <p>{t("workflows.empty")}</p> : null}
				{query.data?.map(w => <button key={w.id} type="button" disabled={busy} className={cn("workflow-library-item", id === w.id && "is-selected")} onClick={() => open(w)}>
					<GitBranch size={15} /><span><strong>{w.name}</strong><small>{t("workflows.stageCount", { count: w.stages.length, mode: t(w.enabled ? "workflows.scheduled" : "workflows.manual") })}</small></span>
				</button>)}
			</aside>
			<main className="workflow-main">
				<div className="workflow-toolbar"><div><h2>{shown.name}</h2><p>{run ? `${t(`workflows.status.${run.status}`, { defaultValue: run.status })} · ${new Date(run.createdAt).toLocaleString()}` : t("workflows.shared")}</p></div>
					<div className="workflow-actions">{runId ? <Button variant="outline" size="sm" onClick={() => { setRunId(undefined); setSelected(0); }}>{t("workflows.edit")}</Button> : <>
						<Button variant="outline" size="sm" disabled={busy} onClick={() => void persist()}><Save />{t(dirty ? "workflows.saveChanges" : "workflows.save")}</Button>
						<Button ref={runButton} size="sm" disabled={busy || !draft.task.trim()} onClick={requestRun}><Play />{t(busy ? "workflows.starting" : "workflows.run")}</Button>
					</>}</div>
				</div>
				{error || history.error ? <p role="alert" className="workflow-error">{error || history.error?.message}</p> : null}
				{!runId ? <div className="workflow-setup">
					<label>{t("workflows.name")}<Input value={draft.name} maxLength={120} onChange={e => update({ name: e.target.value })} /></label>
					<label>{t("workflows.workspace")}<Select value={draft.projectId || "__standalone"} onValueChange={v => update({ projectId: v === "__standalone" ? "" : v })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="__standalone">{t("workflows.standalone")}</SelectItem>{projects.map(p => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}</SelectContent></Select></label>
					<label className="workflow-task">{t("workflows.task")}<textarea rows={3} value={draft.task} maxLength={32000} aria-invalid={!!inputs.error} aria-describedby="workflow-template-help" placeholder={t("workflows.taskPlaceholder")} onChange={e => update({ task: e.target.value })} /></label>
					<div className="workflow-task" id="workflow-template-help"><p className={inputs.error ? "workflow-field-error" : "workflow-muted"}>{inputs.error ? t(inputs.error) : <>{t("workflows.templateHelp")} <code>{"{{company}}"}</code></>}</p>
						<WorkflowDefaults names={inputs.names} draft={draft} update={update} /></div>
				</div> : null}
				{run ? <WorkflowRunInputs run={run} /> : null}
				<div className="workflow-canvas" aria-label={t("workflows.canvas")}>
					{stages.map((stage, i) => <div className="workflow-node-group" key={i}>
						{i > 0 ? <ArrowRight className="workflow-connector" aria-hidden="true" /> : null}
						<button type="button" className={cn("workflow-node", selected === i && "is-selected")} onClick={() => setSelected(i)} aria-pressed={selected === i} aria-label={`${i + 1}. ${stage.name || t("workflows.untitledStage")}`}>
							<span className="workflow-node-number">{String(i + 1).padStart(2, "0")}</span><strong>{stage.name || t("workflows.untitledStage")}</strong><span>{stage.agent}</span>
							<small data-status={run?.stages[i]?.status}>{run ? t(`workflows.status.${run.stages[i].status}`, { defaultValue: run.stages[i].status }) : t(stage.approval && i < stages.length - 1 ? "workflows.checkpoint" : "workflows.automatic")}</small>
						</button>
					</div>)}
					{!runId ? <Button variant="ghost" className="workflow-add" disabled={stages.length >= 30} onClick={() => { update({ stages: [...draft.stages, { name: "New stage", agent: "codex", instructions: "", approval: false }] }); setSelected(draft.stages.length); }}><Plus />{t("workflows.addStage")}</Button> : null}
				</div>
				{run ? <WorkflowRunDetail key={`${run.id}-${selected}`} run={run} index={selected} /> : runId ? <p className="p-5">{t("workflows.loadingRun")}</p> : <WorkflowStageEditor stage={draft.stages[selected]} index={selected} count={stages.length}
					onChange={stage => update({ stages: draft.stages.map((s, i) => i === selected ? stage : s) })}
					onRemove={() => { update({ stages: draft.stages.filter((_, i) => i !== selected) }); setSelected(Math.max(0, selected - 1)); }}
					onMove={delta => { const next = [...draft.stages]; [next[selected], next[selected + delta]] = [next[selected + delta], next[selected]]; update({ stages: next }); setSelected(selected + delta); }} />}
				{!runId ? <WorkflowSchedule draft={draft} update={update} /> : null}
				{history.data?.length ? <section className="workflow-history"><h3>{t("workflows.history")}</h3>{history.data.map(r => <button key={r.id} onClick={() => { setRunId(r.id); setSelected(r.currentStage); }}><span>{new Date(r.createdAt).toLocaleString()}</span><span data-status={r.status}>{t(`workflows.status.${r.status}`, { defaultValue: r.status })}</span><ArrowRight size={14} /></button>)}</section> : null}
			</main>
		</div>
		{inputDialog ? <WorkflowInputDialog name={draft.name} names={inputs.names} defaults={draft.inputDefaults} onClose={() => setInputDialog(false)} onRun={values => persist(true, values)} onRestoreFocus={() => runButton.current?.focus()} /> : null}
	</div>;
}
