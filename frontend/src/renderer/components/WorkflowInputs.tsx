import { useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Play } from "lucide-react";
import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, settingsDialogContentClass, settingsDialogHeaderClass, settingsDialogBodyClass, settingsDialogFooterClass } from "./ui/dialog";
import { inputValue, WorkflowInputError } from "../lib/workflow-inputs";
import type { WorkflowInput, WorkflowRun } from "../hooks/useWorkflows";

export function WorkflowDefaults({ names, draft, update }: {
	names: string[]; draft: WorkflowInput; update: (value: Partial<WorkflowInput>) => void;
}) {
	const { t } = useTranslation();
	const id = useId();
	if (!names.length) return null;
	return <details className="workflow-defaults">
		<summary>{t("workflows.inputDefaults", { count: names.length })}</summary>
		<p className="workflow-muted">{t("workflows.defaultsHelp")}</p>
		<div className="workflow-default-fields">{names.map((name, index) => <label key={name} htmlFor={`${id}-${index}`}>
			<span>{name} <small>{t("workflows.optionalDefault")}</small></span>
			<textarea id={`${id}-${index}`} rows={2} maxLength={32000} value={inputValue(draft.inputDefaults, name)}
				onChange={e => update({ inputDefaults: { ...draft.inputDefaults, [name]: e.target.value } })} />
		</label>)}</div>
	</details>;
}

export function WorkflowInputDialog({ name, names, defaults, onClose, onRun, onRestoreFocus }: {
	name: string; names: string[]; defaults?: Record<string, string>; onClose: () => void;
	onRun: (inputs: Record<string, string>) => Promise<void>;
	onRestoreFocus?: () => void;
}) {
	const { t } = useTranslation();
	const id = useId();
	const [values, setValues] = useState<Record<string, string>>(() => Object.fromEntries(names.map(key => [key, inputValue(defaults, key)])));
	const [errors, setErrors] = useState<Record<string, string>>({});
	const [error, setError] = useState("");
	const [busy, setBusy] = useState(false);
	const pending = useRef(false);
	const form = useRef<HTMLFormElement>(null);
	async function submit() {
		if (pending.current) return;
		const missing = Object.fromEntries(names.filter(key => !values[key]?.trim()).map(key => [key, t("workflows.inputRequired")]));
		setErrors(missing); setError("");
		if (Object.keys(missing).length) {
			requestAnimationFrame(() => form.current?.querySelector<HTMLTextAreaElement>('[aria-invalid="true"]')?.focus());
			return;
		}
		pending.current = true; setBusy(true);
		try { await onRun(values); }
		catch (e) {
			setError(e instanceof Error ? e.message : t("workflows.startFailed"));
			if (e instanceof WorkflowInputError) setErrors(e.fields);
		} finally { pending.current = false; setBusy(false); }
	}
	return <Dialog open onOpenChange={open => { if (!open && !pending.current) onClose(); }}>
		<DialogContent className={`${settingsDialogContentClass} workflow-input-dialog`} showCloseButton={!busy}
			onCloseAutoFocus={e => { if (onRestoreFocus) { e.preventDefault(); onRestoreFocus(); } }}>
			<DialogHeader className={settingsDialogHeaderClass}>
				<DialogTitle>{t("workflows.run")}</DialogTitle>
				<DialogDescription>{t("workflows.runInputsHelp", { name })}</DialogDescription>
			</DialogHeader>
			<form ref={form} className="workflow-input-form" noValidate onSubmit={e => { e.preventDefault(); void submit(); }}>
				<div className={settingsDialogBodyClass}>
					{names.map((key, index) => <label className="workflow-input-field" htmlFor={`${id}-${index}`} key={key}>
						<span>{key}<small>{t("workflows.required")}</small></span>
						<textarea id={`${id}-${index}`} rows={3} maxLength={32000} required disabled={busy} value={values[key] ?? ""}
							aria-invalid={!!inputValue(errors, key)} aria-describedby={inputValue(errors, key) ? `${id}-${index}-error` : undefined}
							onChange={e => { setValues(v => ({ ...v, [key]: e.target.value })); setErrors(v => ({ ...v, [key]: "" })); }} />
						{inputValue(errors, key) ? <span className="workflow-field-error" id={`${id}-${index}-error`}>{inputValue(errors, key)}</span> : null}
					</label>)}
					{error ? <p role="alert" className="workflow-error">{error}</p> : null}
				</div>
				<div className={settingsDialogFooterClass}>
					<Button type="button" variant="outline" disabled={busy} onClick={onClose}>{t("workflows.cancel")}</Button>
					<Button type="submit" disabled={busy}><Play />{t(busy ? "workflows.starting" : "workflows.run")}</Button>
				</div>
			</form>
		</DialogContent>
	</Dialog>;
}

export function WorkflowRunInputs({ run }: { run: WorkflowRun }) {
	const { t } = useTranslation();
	return <details className="workflow-run-inputs">
		<summary>{t("workflows.inputsAndPrompt")}</summary>
		{Object.keys(run.inputs ?? {}).length ? <dl>{Object.entries(run.inputs ?? {}).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl> : null}
		<h3>{t("workflows.resolvedPrompt")}</h3><p className="workflow-handoff">{run.resolvedTask ?? run.definition.task}</p>
	</details>;
}
