// Keep discovery aligned with the daemon's workflow/inputs.go parser.
export function parseWorkflowInputs(task: string): { names: string[]; error?: "workflows.unclosedInput" | "workflows.invalidInputName" } {
	const names: string[] = [];
	for (let offset = 0; offset < task.length;) {
		const start = task.indexOf("{{", offset);
		if (start < 0) break;
		const end = task.indexOf("}}", start + 2);
		if (end < 0) return { names, error: "workflows.unclosedInput" };
		const name = task.slice(start + 2, end).trim();
		if (!name || /[{}\r\n]/.test(name)) return { names, error: "workflows.invalidInputName" };
		if (!names.includes(name)) names.push(name);
		offset = end + 2;
	}
	return { names };
}

export function inputValue(values: Record<string, string> | undefined, name: string): string {
	return values && Object.hasOwn(values, name) ? values[name] : "";
}

export class WorkflowInputError extends Error {
	constructor(message: string, public fields: Record<string, string> = {}) { super(message); }
}

export function workflowErrorFields(error: unknown): Record<string, string> {
	const body = error as { details?: { fields?: unknown } } | null;
	const fields = body?.details?.fields;
	if (!fields || typeof fields !== "object") return {};
	return Object.fromEntries(Object.entries(fields).filter((entry): entry is [string, string] => typeof entry[1] === "string"));
}
