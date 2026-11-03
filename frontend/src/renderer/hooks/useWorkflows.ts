import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { components } from "../../api/schema";
import { apiClient, apiErrorMessage } from "../lib/api-client";
import { WorkflowInputError, workflowErrorFields } from "../lib/workflow-inputs";

export type Workflow = components["schemas"]["Workflow"];
export type WorkflowStage = components["schemas"]["WorkflowStage"];
export type WorkflowRun = components["schemas"]["WorkflowRun"];
export type WorkflowInput = components["schemas"]["SaveWorkflowRequest"];

export function useWorkflows() {
	return useQuery({ queryKey: ["workflows"], refetchInterval: 5000, queryFn: async () => {
		const { data, error } = await apiClient.GET("/api/v1/workflows");
		if (error) throw new Error(apiErrorMessage(error));
		return data?.workflows ?? [];
	} });
}

export function useWorkflowRuns(id?: string) {
	return useQuery({ queryKey: ["workflow-runs", id], enabled: !!id, refetchInterval: 2000, queryFn: async () => {
		const { data, error } = await apiClient.GET("/api/v1/workflows/{workflowId}/runs", { params: { path: { workflowId: id! } } });
		if (error) throw new Error(apiErrorMessage(error));
		return data?.runs ?? [];
	} });
}

export function useSaveWorkflow() {
	const client = useQueryClient();
	return useMutation({ mutationFn: async ({ id, body }: { id?: string; body: WorkflowInput }) => {
		// Saved definitions also carry server-owned fields. Send only editable
		// fields so reopening or saving a definition again passes strict decoding.
		const { name, projectId, task, inputDefaults, stages, rrule, timezone, enabled } = body;
		const input: WorkflowInput = { name, projectId, task, inputDefaults, stages, rrule, timezone, enabled };
		const result = id
			? await apiClient.PUT("/api/v1/workflows/{workflowId}", { params: { path: { workflowId: id } }, body: input })
			: await apiClient.POST("/api/v1/workflows", { body: input });
		if (result.error) throw new WorkflowInputError(apiErrorMessage(result.error), workflowErrorFields(result.error));
		if (!result.data) throw new Error("Workflow response was empty");
		return result.data.workflow;
	}, onSuccess: () => client.invalidateQueries({ queryKey: ["workflows"] }) });
}

export function useStartWorkflow() {
	const client = useQueryClient();
	return useMutation({ mutationFn: async ({ id, inputs }: { id: string; inputs?: Record<string, string> }) => {
		const { data, error } = await apiClient.POST("/api/v1/workflows/{workflowId}/runs", { params: { path: { workflowId: id } }, body: { inputs } });
		if (error) throw new WorkflowInputError(apiErrorMessage(error), workflowErrorFields(error));
		if (!data) throw new Error("Run response was empty");
		return data.run;
	}, onSuccess: () => client.invalidateQueries({ queryKey: ["workflow-runs"] }) });
}

export function useWorkflowAction() {
	const client = useQueryClient();
	return useMutation({ mutationFn: async ({ id, action, feedback }: { id: string; action: "approve" | "retry"; feedback?: string }) => {
		const { data, error } = await apiClient.POST("/api/v1/workflow-runs/{runId}/actions", { params: { path: { runId: id } }, body: { action, feedback } });
		if (error) throw new Error(apiErrorMessage(error));
		return data?.run;
	}, onSuccess: () => client.invalidateQueries({ queryKey: ["workflow-runs"] }) });
}
