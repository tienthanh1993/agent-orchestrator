import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { expect, it, vi } from "vitest";
import { useSaveWorkflow, useStartWorkflow, type Workflow } from "./useWorkflows";

const api = vi.hoisted(() => ({ POST: vi.fn(), PUT: vi.fn() }));
vi.mock("../lib/api-client", () => ({ apiClient: api, apiErrorMessage: (e: { message: string }) => e.message }));

it("can save a reopened scheduled definition without sending server-owned fields", async () => {
	const saved: Workflow = {
		id: "workflow-1", name: "Documentation release", projectId: "", task: "Prepare a release checklist",
		inputDefaults: { market: "US" },
		stages: [{ name: "Research", agent: "codex", instructions: "Save research.md", approval: false }],
		enabled: true, rrule: "FREQ=DAILY;BYHOUR=9", timezone: "UTC", nextRunAt: "2026-11-03T09:00:00Z",
	};
	api.PUT.mockImplementation(async (_path, { body }) => {
		if ("id" in body || "nextRunAt" in body) return { error: { message: "Invalid workflow body" } };
		return { data: { workflow: { ...saved, ...body } } };
	});
	const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
	const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
	const { result } = renderHook(() => useSaveWorkflow(), { wrapper });
	let updated: Workflow | undefined;
	await act(async () => { updated = await result.current.mutateAsync({ id: saved.id, body: { ...saved, name: "Updated checklist" } }); });
	expect(updated?.name).toBe("Updated checklist");
	expect(updated?.id).toBe(saved.id);
	expect(updated?.nextRunAt).toBe(saved.nextRunAt);
	expect(api.PUT).toHaveBeenCalledWith("/api/v1/workflows/{workflowId}", expect.objectContaining({
		params: { path: { workflowId: saved.id } },
		body: expect.objectContaining({ enabled: true, stages: saved.stages, name: "Updated checklist", inputDefaults: { market: "US" } }),
	}));
	client.clear();
});

it("sends run inputs and retains field-specific validation errors", async () => {
	api.POST.mockResolvedValue({ error: { message: "Fill in every workflow input", details: { fields: { company: "Enter a value" } } } });
	const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
	const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
	const { result } = renderHook(() => useStartWorkflow(), { wrapper });
	await act(async () => {
		await expect(result.current.mutateAsync({ id: "workflow-1", inputs: { company: "" } })).rejects.toMatchObject({ fields: { company: "Enter a value" } });
	});
	expect(api.POST).toHaveBeenCalledWith("/api/v1/workflows/{workflowId}/runs", { params: { path: { workflowId: "workflow-1" } }, body: { inputs: { company: "" } } });
	client.clear();
});
