import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { WorkflowInputDialog } from "./WorkflowInputs";
import { WorkflowInputError } from "../lib/workflow-inputs";

it("validates blank values, preserves multiline text on failure, and blocks duplicate submissions", async () => {
	const user = userEvent.setup();
	let finish: () => void = () => {};
	const onRun = vi.fn().mockRejectedValueOnce(new WorkflowInputError("Input rejected", { company: "Shorten this value" }))
		.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
	const onClose = vi.fn();
	render(<WorkflowInputDialog name="Market research" names={["company"]} defaults={{ company: "   " }} onClose={onClose} onRun={onRun} />);
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	expect(onRun).not.toHaveBeenCalled();
	expect(screen.getByText("Enter a value")).toBeVisible();
	const field = screen.getByRole("textbox", { name: /company/ });
	fireEvent.change(field, { target: { value: "Acme\n$1 <tag> {{literal}}" } });
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Input rejected"));
	expect(screen.getByText("Shorten this value")).toBeVisible();
	expect(field).toHaveValue("Acme\n$1 <tag> {{literal}}");
	fireEvent.change(field, { target: { value: "Acme" } });
	await user.click(screen.getByRole("button", { name: "Run workflow" }));
	expect(screen.getByRole("button", { name: "Starting…" })).toBeDisabled();
	expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
	fireEvent.submit(field.closest("form")!);
	expect(onRun).toHaveBeenCalledTimes(2);
	await act(async () => finish());
});
