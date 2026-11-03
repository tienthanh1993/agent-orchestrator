import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { WorkflowsView } from "./WorkflowsView";
import { WorkflowRunDetail } from "./WorkflowDetails";
import type { WorkflowInput, WorkflowRun } from "../hooks/useWorkflows";

const mocks = vi.hoisted(() => ({ save: vi.fn(), start: vi.fn(), act: vi.fn(), navigate: vi.fn(), openFile: vi.fn() }));
vi.mock("../hooks/useWorkflows", () => ({
	useWorkflows: () => ({ data: [], isLoading: false }),
	useSaveWorkflow: () => ({ mutateAsync: mocks.save, isPending: false }),
	useStartWorkflow: () => ({ mutateAsync: mocks.start, isPending: false }),
	useWorkflowRuns: () => ({ data: [] }),
	useWorkflowAction: () => ({ mutate: mocks.act, isPending: false }),
}));
vi.mock("../hooks/useWorkspaceQuery", () => ({ useWorkspaceQuery: () => ({ data: [] }) }));
vi.mock("../hooks/useSettings", () => ({ useSettings: () => ({ settings: { chatHarnesses: ["codex", "claude-code"] } }) }));
vi.mock("../hooks/useAgentReadinessQuery", () => ({ useAgentReadinessQuery: () => ({ data: { agents: [{ id: "codex", label: "Codex" }, { id: "claude-code", label: "Claude Code" }] } }) }));
vi.mock("../lib/navigate-to-session", () => ({ useNavigateToSession: () => mocks.navigate }));
vi.mock("../stores/ui-store", () => ({ useUiStore: (select: (s: unknown) => unknown) => select({ requestWorkspaceFileOpen: mocks.openFile }) }));

beforeEach(() => { vi.clearAllMocks(); });

it("collects unique inputs, cancels without saving, and keeps manual values out of defaults", async () => {
	const user = userEvent.setup();
	mocks.save.mockImplementation(async ({ body }: { body: WorkflowInput }) => ({ ...body, id: "saved" }));
	mocks.start.mockResolvedValue({ id: "new-run" });
	render(<WorkflowsView />);
	fireEvent.change(screen.getByLabelText("Master prompt"), { target: { value: "Research {{ company }} in {{market}} for {{company}}" } });
	expect(screen.getByLabelText("Run on a schedule")).toBeDisabled();
	await user.click(screen.getByText("Input defaults (2)"));
	fireEvent.change(screen.getByLabelText("company Optional default"), { target: { value: "Saved company" } });
	fireEvent.change(screen.getByLabelText("market Optional default"), { target: { value: "US" } });
	expect(screen.getByLabelText("Run on a schedule")).toBeEnabled();
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	let dialog = within(screen.getByRole("dialog"));
	expect(dialog.getAllByRole("textbox")).toHaveLength(2);
	expect(dialog.getByRole("textbox", { name: /company/ })).toHaveValue("Saved company");
	await user.click(dialog.getByRole("button", { name: "Cancel" }));
	expect(mocks.save).not.toHaveBeenCalled();
	expect(mocks.start).not.toHaveBeenCalled();
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	dialog = within(screen.getByRole("dialog"));
	fireEvent.change(dialog.getByRole("textbox", { name: /company/ }), { target: { value: "Override\n{{literal}}" } });
	await user.click(dialog.getByRole("button", { name: "Run workflow" }));
	await waitFor(() => expect(mocks.start).toHaveBeenCalledWith({ id: "saved", inputs: { company: "Override\n{{literal}}", market: "US" } }));
	expect(mocks.save.mock.calls[0][0].body.inputDefaults).toEqual({ company: "Saved company", market: "US" });
});

it("rejects malformed templates before saving or starting", async () => {
	const user = userEvent.setup();
	render(<WorkflowsView />);
	fireEvent.change(screen.getByLabelText("Master prompt"), { target: { value: "Research {{company" } });
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	expect(screen.getByRole("alert")).toHaveTextContent("Close each input placeholder");
	expect(mocks.start).not.toHaveBeenCalled();
	expect(mocks.save).not.toHaveBeenCalled();
});

function run(status: string): WorkflowRun {
	return {
		id: "run-1", workflowId: "workflow-1", sessionId: "session-1", status, currentStage: 1, createdAt: "2026-11-02T09:00:00Z",
		definition: { id: "workflow-1", name: "Offline reader", task: "Build the reader", projectId: "reader", enabled: false, stages: [
			{ name: "Research", agent: "codex", instructions: "Research", approval: false },
			{ name: "Plan", agent: "claude-code", instructions: "Plan", approval: true },
			{ name: "Build", agent: "codex", instructions: "Build", approval: false },
		] },
		stages: [
			{ status: "completed", attempt: 1, files: [], handoff: "Research ready" },
			{ status, attempt: 2, handoff: "Cache selected. Implement the plan.", files: [{ path: "workflow/02/plan.md", name: "plan.md", kind: "markdown", size: 100, updatedAt: "2026-11-02T09:00:00Z" }] },
			{ status: "pending", attempt: 1, files: [] },
		],
	};
}

it("saves the edited stage instructions and schedule before starting the workflow", async () => {
	const user = userEvent.setup();
	mocks.save.mockImplementation(async ({ body }: { body: WorkflowInput }) => ({ ...body, id: "saved" }));
	mocks.start.mockResolvedValue({ id: "new-run" });
	render(<WorkflowsView />);
	await user.type(screen.getByLabelText("Master prompt"), "Build an offline reader");
	await user.click(screen.getByRole("button", { name: "2. Plan" }));
	await user.clear(screen.getByLabelText("Instructions"));
	await user.type(screen.getByLabelText("Instructions"), "Read research.md and save plan.md");
	await user.click(screen.getByLabelText("Run on a schedule"));
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	await waitFor(() => expect(mocks.start).toHaveBeenCalledWith({ id: "saved", inputs: undefined }));
	const saved = mocks.save.mock.calls[0][0].body as WorkflowInput;
	expect(saved.task).toBe("Build an offline reader");
	expect(saved.stages[1].instructions).toBe("Read research.md and save plan.md");
	expect(saved.stages[1].approval).toBe(true);
	expect(saved.enabled).toBe(true);
	expect(saved.rrule).toContain("FREQ=DAILY");
});

it("approves only the current checkpoint and opens files in the shared session", async () => {
	const user = userEvent.setup();
	const { rerender } = render(<WorkflowRunDetail run={run("awaiting_approval")} index={0} />);
	expect(screen.queryByRole("button", { name: "Approve and continue" })).not.toBeInTheDocument();
	rerender(<WorkflowRunDetail run={run("awaiting_approval")} index={1} />);
	await user.click(screen.getByRole("button", { name: "Approve and continue" }));
	expect(mocks.act).toHaveBeenCalledWith({ id: "run-1", action: "approve" });
	await user.click(screen.getByRole("button", { name: "plan.md" }));
	expect(mocks.openFile).toHaveBeenCalledWith("session-1", "workflow/02/plan.md", undefined, "artifact");
	expect(mocks.navigate).toHaveBeenCalledWith("reader", "session-1");
});

it("sends retry feedback for the failed stage", async () => {
	const user = userEvent.setup();
	render(<WorkflowRunDetail run={run("failed")} index={1} />);
	await user.type(screen.getByLabelText("Feedback for retry"), "Include cache invalidation");
	await user.click(screen.getByRole("button", { name: "Retry stage" }));
	expect(mocks.act).toHaveBeenCalledWith({ id: "run-1", action: "retry", feedback: "Include cache invalidation" });
	expect(screen.getByText("Earlier stages and saved files are retained.")).toBeInTheDocument();
});
