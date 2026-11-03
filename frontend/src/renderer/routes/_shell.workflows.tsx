import { createFileRoute } from "@tanstack/react-router";
import { WorkflowsView } from "../components/WorkflowsView";

export const Route = createFileRoute("/_shell/workflows")({ component: WorkflowsView });
