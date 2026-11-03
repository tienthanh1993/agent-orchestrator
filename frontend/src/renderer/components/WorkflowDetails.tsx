import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigateToSession } from "../lib/navigate-to-session";
import { ArrowLeft, ArrowRight, Check, FileText, RotateCcw, Trash2 } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";
import { useAgentReadinessQuery } from "../hooks/useAgentReadinessQuery";
import { useSettings } from "../hooks/useSettings";
import { useWorkflowAction, type WorkflowInput, type WorkflowRun, type WorkflowStage } from "../hooks/useWorkflows";
import { useUiStore } from "../stores/ui-store";
import { inputValue, parseWorkflowInputs } from "../lib/workflow-inputs";

export function WorkflowStageEditor({ stage, index, count, onChange, onRemove, onMove }: {
	stage: WorkflowStage; index: number; count: number; onChange: (s: WorkflowStage) => void; onRemove: () => void; onMove: (delta: number) => void;
}) {
	const { t } = useTranslation();
	const agents = useAgentReadinessQuery().data?.agents ?? [];
	const { settings } = useSettings();
	const available = agents.filter(a => settings?.chatHarnesses.includes(a.id));
	const choices = available.some(a => a.id === stage.agent) ? available : [{ id: stage.agent, label: stage.agent }, ...available];
	return <section className="workflow-stage-editor" aria-label={t("workflows.stageEditor")}>
		<div className="workflow-detail-heading"><div><h3>{t("workflows.stageNumber", { number: index + 1 })}</h3><p>{t("workflows.stageHelp")}</p></div><div className="workflow-actions">
			<Button size="icon-sm" variant="ghost" aria-label={t("workflows.moveEarlier")} disabled={index === 0} onClick={() => onMove(-1)}><ArrowLeft /></Button>
			<Button size="icon-sm" variant="ghost" aria-label={t("workflows.moveLater")} disabled={index === count - 1} onClick={() => onMove(1)}><ArrowRight /></Button>
			<Button size="icon-sm" variant="ghost" aria-label={t("workflows.removeStage")} disabled={count === 1} onClick={onRemove}><Trash2 /></Button>
		</div></div>
		<div className="workflow-setup">
			<label>{t("workflows.stageName")}<Input maxLength={120} value={stage.name} onChange={e => onChange({ ...stage, name: e.target.value })} /></label>
			<label>{t("workflows.agent")}<Select value={stage.agent} onValueChange={agent => onChange({ ...stage, agent, model: "", effort: "" })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{choices.map(a => <SelectItem key={a.id} value={a.id}>{a.label}</SelectItem>)}</SelectContent></Select></label>
			<label className="workflow-task">{t("workflows.instructions")}<textarea rows={5} maxLength={32000} value={stage.instructions} onChange={e => onChange({ ...stage, instructions: e.target.value })} placeholder={t("workflows.instructionsPlaceholder")} /></label>
		</div>
		<div className="workflow-stage-options"><label className="workflow-check"><input type="checkbox" checked={stage.approval} disabled={index === count - 1} onChange={e => onChange({ ...stage, approval: e.target.checked })} />{t("workflows.approval")}</label>
			<details><summary>{t("workflows.modelSettings")}</summary><div className="workflow-setup"><label>{t("workflows.model")}<Input value={stage.model ?? ""} placeholder={t("workflows.default")} onChange={e => onChange({ ...stage, model: e.target.value })} /></label><label>{t("workflows.effort")}<Input value={stage.effort ?? ""} placeholder={t("workflows.default")} onChange={e => onChange({ ...stage, effort: e.target.value })} /></label></div></details>
		</div>
	</section>;
}

export function WorkflowSchedule({ draft, update }: { draft: WorkflowInput; update: (d: Partial<WorkflowInput>) => void }) {
	const { t } = useTranslation();
	const inputs = parseWorkflowInputs(draft.task);
	const missingDefaults = !!inputs.error || inputs.names.some(name => !inputValue(draft.inputDefaults, name).trim());
	const rule = draft.rrule ?? "";
	const hour = /BYHOUR=(\d+)/.exec(rule)?.[1]?.padStart(2, "0") ?? "09";
	const minute = /BYMINUTE=(\d+)/.exec(rule)?.[1]?.padStart(2, "0") ?? "00";
	const weekdays = rule.includes("BYDAY=MO,TU,WE,TH,FR");
	function schedule(enabled: boolean, time = `${hour}:${minute}`, week = weekdays) {
		const [h, m] = time.split(":").map(Number);
		if (!Number.isFinite(h) || !Number.isFinite(m)) return;
		update({ enabled, rrule: `FREQ=DAILY;BYHOUR=${h};BYMINUTE=${m};BYSECOND=0${week ? ";BYDAY=MO,TU,WE,TH,FR" : ""}` });
	}
	return <section className="workflow-schedule"><label className="workflow-check"><input type="checkbox" checked={draft.enabled} disabled={!draft.enabled && missingDefaults} onChange={e => schedule(e.target.checked)} />{t("workflows.schedule")}</label>
		{missingDefaults ? <p className="workflow-muted">{t("workflows.scheduleDefaultsHelp")}</p> : null}
		{draft.enabled ? <div className="workflow-schedule-fields"><label>{t("workflows.repeat")}<Select value={weekdays ? "weekdays" : "daily"} onValueChange={v => schedule(true, `${hour}:${minute}`, v === "weekdays")}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="daily">{t("workflows.daily")}</SelectItem><SelectItem value="weekdays">{t("workflows.weekdays")}</SelectItem></SelectContent></Select></label><label>{t("workflows.time")}<Input type="time" value={`${hour}:${minute}`} onChange={e => schedule(true, e.target.value)} /></label><label>{t("workflows.timezone")}<Input value={draft.timezone ?? "UTC"} onChange={e => update({ timezone: e.target.value })} /></label></div> : null}
	</section>;
}

export function WorkflowRunDetail({ run, index }: { run: WorkflowRun; index: number }) {
	const { t } = useTranslation();
	const action = useWorkflowAction();
	const navigateToSession = useNavigateToSession();
	const requestWorkspaceFileOpen = useUiStore(s => s.requestWorkspaceFileOpen);
	const [feedback, setFeedback] = useState("");
	const state = run.stages[index];
	const current = run.currentStage === index;
	return <section className="workflow-run-detail">
		<div className="workflow-detail-heading"><div><h3>{run.definition.stages[index].name}</h3><p>{t("workflows.attempt", { number: state.attempt })} · <span data-status={state.status}>{t(`workflows.status.${state.status}`, { defaultValue: state.status })}</span></p></div>
			{run.sessionId ? <Button variant="outline" size="sm" onClick={() => navigateToSession(run.definition.projectId, run.sessionId!)}>{t("workflows.conversation")}</Button> : null}</div>
		{state.error ? <p className="workflow-error" role="alert">{state.error}</p> : null}
		<div className="workflow-handoff-grid"><div><h4>{t("workflows.handoff")}</h4>{state.handoff ? <p className="workflow-handoff">{state.handoff}</p> : <p className="workflow-muted">{t(state.status === "pending" ? "workflows.inheritance" : "workflows.waitingHandoff")}</p>}</div>
			<div><h4>{t("workflows.files")}</h4>{state.files.length ? state.files.map(file => <button type="button" className="workflow-file" key={file.path} onClick={() => {
				if (!run.sessionId) return;
				requestWorkspaceFileOpen(run.sessionId, file.path, undefined, "artifact");
				navigateToSession(run.definition.projectId, run.sessionId);
			}}><FileText size={14} /><span>{file.name}</span></button>) : <p className="workflow-muted">{t("workflows.noFiles")}</p>}</div></div>
		{current && run.status === "awaiting_approval" ? <div className="workflow-decision"><p>{t("workflows.review")}</p><Button disabled={action.isPending} onClick={() => action.mutate({ id: run.id, action: "approve" })}><Check />{t("workflows.approve")}</Button></div> : null}
		{current && run.status === "failed" ? <div className="workflow-decision"><label>{t("workflows.feedback")}<textarea rows={3} value={feedback} maxLength={16000} onChange={e => setFeedback(e.target.value)} placeholder={t("workflows.feedbackPlaceholder")} /></label><Button disabled={action.isPending} onClick={() => action.mutate({ id: run.id, action: "retry", feedback })}><RotateCcw />{t("workflows.retry")}</Button><p>{t("workflows.retained")}</p></div> : null}
		{action.error ? <p role="alert" className="workflow-error">{action.error.message}</p> : null}
	</section>;
}
