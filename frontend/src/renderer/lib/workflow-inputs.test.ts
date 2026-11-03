import { expect, it } from "vitest";
import { inputValue, parseWorkflowInputs, workflowErrorFields } from "./workflow-inputs";

it("discovers case-sensitive names in appearance order and reuses repeated fields", () => {
	expect(parseWorkflowInputs("{{ company }} {{market}} {{company}} {{Company}}").names).toEqual(["company", "market", "Company"]);
	expect(parseWorkflowInputs("ordinary {JSON} prompt")).toEqual({ names: [] });
	for (const task of ["{{}}", "{{ }}", "{{name", "{{outer {{inner}}", "{{first\nlast}}"])
		expect(parseWorkflowInputs(task).error).toBeTruthy();
	expect(inputValue({}, "__proto__")).toBe("");
	expect(workflowErrorFields({ details: { fields: { company: "Enter a value" } } })).toEqual({ company: "Enter a value" });
});
