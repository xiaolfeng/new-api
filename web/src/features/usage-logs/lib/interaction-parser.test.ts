import { describe, it, expect } from "vitest";

import {
  normalizeInteractionType,
  parseInteractionType,
  resolveInteractionTypeFromLog,
} from "./interaction-parser";

describe("resolveInteractionTypeFromLog", () => {
  it("prefers precomputed other.interaction_type", () => {
    expect(
      resolveInteractionTypeFromLog({
        other: JSON.stringify({ interaction_type: "callback" }),
        content: "",
      }),
    ).toBe("callback");
  });

  it("resolves chinese single turn values from other", () => {
    expect(
      resolveInteractionTypeFromLog({
        other: JSON.stringify({ interaction_type: "单轮" }),
        content: "",
      }),
    ).toBe("single_turn");
  });

  it("falls back to content parsing", () => {
    expect(
      resolveInteractionTypeFromLog({
        other: "",
        content: JSON.stringify({
          claudeRequestBlocks: [{ type: "text", text: "Hello" }],
          claudeToolResponses: [],
          claudeResponseBlocks: [],
        }),
      }),
    ).toBe("input");
  });
});

describe("normalizeInteractionType", () => {
  it("accepts english and chinese precomputed values", () => {
    expect(normalizeInteractionType("input")).toBe("input");
    expect(normalizeInteractionType("输入")).toBe("input");
    expect(normalizeInteractionType("output")).toBe("output");
    expect(normalizeInteractionType("输出")).toBe("output");
    expect(normalizeInteractionType("callback")).toBe("callback");
    expect(normalizeInteractionType("回调")).toBe("callback");
    expect(normalizeInteractionType("single_turn")).toBe("single_turn");
    expect(normalizeInteractionType("单轮")).toBe("single_turn");
  });

  it("ignores unknown values", () => {
    expect(normalizeInteractionType("other")).toBeUndefined();
    expect(normalizeInteractionType(null)).toBeUndefined();
  });
});

describe("parseInteractionType", () => {
  it("returns null for null input", () => {
    expect(parseInteractionType(null)).toBeNull();
  });

  it("returns null for undefined input", () => {
    expect(parseInteractionType(undefined)).toBeNull();
  });

  it("returns null for empty object", () => {
    expect(parseInteractionType({})).toBeNull();
  });

  it("detects input type from claude request blocks", () => {
    const record = {
      claudeRequestBlocks: [{ type: "text", text: "Hello" }],
      claudeToolResponses: [],
      claudeResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("detects output type from claude response blocks", () => {
    const record = {
      claudeRequestBlocks: [],
      claudeToolResponses: [],
      claudeResponseBlocks: [{ type: "text", content: "Response text here" }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("detects callback type from tool responses", () => {
    const record = {
      claudeRequestBlocks: [],
      claudeToolResponses: [
        { toolUseId: "1", name: "tool1", type: "tool_result" },
      ],
      claudeResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects callback from tool_use in response blocks", () => {
    const record = {
      claudeRequestBlocks: [],
      claudeToolResponses: [],
      claudeResponseBlocks: [
        { type: "tool_use", id: "1", name: "tool1", input: {} },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects input from openai request blocks", () => {
    const record = {
      openaiRequestBlocks: [{ type: "text", role: "user", text: "Hello" }],
      openaiToolResponses: [],
      openaiResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("handles JSON string input", () => {
    const record = JSON.stringify({
      claudeRequestBlocks: [{ type: "text", text: "Hello" }],
    });
    expect(parseInteractionType(record)).toBe("input");
  });

  it("handles responses format with request blocks", () => {
    const record = {
      responsesRequestBlocks: [
        { type: "input_text", text: "Hello", role: "user" },
      ],
      responsesToolResponses: [],
      responsesResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("handles prompt field fallback", () => {
    const record = {
      prompt: { lastUserMessage: { content: "Hello" } },
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("detects output from responses format", () => {
    const record = {
      responsesRequestBlocks: [],
      responsesToolResponses: [],
      responsesResponseBlocks: [
        { type: "output_text", content: "AI response" },
      ],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("detects callback from responses function_call", () => {
    const record = {
      responsesRequestBlocks: [],
      responsesToolResponses: [],
      responsesResponseBlocks: [
        { type: "function_call", name: "tool1", arguments: "{}" },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects single turn from openai request blocks with reasoning-only output", () => {
    const record = {
      openaiRequestBlocks: [{ type: "text", role: "user", text: "Hello" }],
      openaiToolResponses: [],
      openaiResponseBlocks: [{ type: "reasoning", content: "Thinking..." }],
    };
    expect(parseInteractionType(record)).toBe("single_turn");
  });

  it("detects output from openai structured response blocks", () => {
    const record = {
      openaiRequestBlocks: [],
      openaiToolResponses: [],
      openaiResponseBlocks: [{ type: "content", content: "Done" }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("prioritizes tool calls over request input as callback", () => {
    const record = {
      openaiRequestBlocks: [
        { type: "text", role: "user", text: "Run command" },
      ],
      openaiToolResponses: [],
      openaiResponseBlocks: [
        { type: "content", content: "I will run it." },
        { type: "tool_call", id: "call_1", name: "exec_command" },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects openai tool response with text output as output", () => {
    const record = {
      openaiRequestBlocks: [],
      openaiToolResponses: [
        { type: "tool", role: "tool", toolCallId: "call_1" },
      ],
      openaiResponseBlocks: [{ type: "content", content: "Task done" }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("detects openai tool responses as callback", () => {
    const record = {
      openaiRequestBlocks: [],
      openaiToolResponses: [
        { type: "tool", role: "tool", toolCallId: "call_1" },
      ],
      openaiResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("returns null for invalid JSON string", () => {
    expect(parseInteractionType("not-json")).toBeNull();
  });

  it("handles prompt as string", () => {
    const record = {
      prompt: "Hello world",
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("handles responses format with tool responses", () => {
    const record = {
      responsesRequestBlocks: [],
      responsesToolResponses: [
        { type: "function_call_output", output: "result" },
      ],
      responsesResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects bamboo input from request blocks", () => {
    const record = {
      bambooRequestBlocks: [{ text: "Hello bamboo" }],
      bambooToolResponses: [],
      bambooResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("detects bamboo output from response text blocks", () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [],
      bambooResponseBlocks: [{ type: "text", text: "Bamboo response" }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("detects bamboo callback from bare tool responses (no text, no tool_use)", () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [{ toolUseId: "1", output: "result" }],
      bambooResponseBlocks: [],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects bamboo output when tool response exists but AI gives final text", () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [{ toolUseId: "1", output: "result" }],
      bambooResponseBlocks: [{ type: "text", text: "Final answer" }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("detects bamboo callback when tool_use follows tool response", () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [{ toolUseId: "1", output: "result" }],
      bambooResponseBlocks: [
        { type: "text", text: "Let me check" },
        { type: "tool_use", id: "2", name: "tool2", input: {} },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects bamboo callback from tool_use in response blocks", () => {
    const record = {
      bambooRequestBlocks: [],
      bambooToolResponses: [],
      bambooResponseBlocks: [
        { type: "tool_use", id: "1", name: "tool1", input: {} },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("does not let prompt fallback short-circuit bamboo output", () => {
    const record = {
      prompt: { lastUserMessage: { content: "User message" } },
      bambooRequestBlocks: [],
      bambooToolResponses: [],
      bambooResponseBlocks: [{ type: "text", text: "Bamboo output" }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("does not let leftover claude request text force input on tool turns", () => {
    const record = {
      claudeRequestBlocks: [{ type: "text", text: "original user prompt" }],
      claudeToolResponses: [
        { toolUseId: "1", name: "Read", type: "tool_result" },
      ],
      claudeResponseBlocks: [{ type: "text", content: "Here is the file." }],
    };
    expect(parseInteractionType(record)).toBe("output");
  });

  it("detects claude callback when leftover request text accompanies tool_use", () => {
    const record = {
      claudeRequestBlocks: [{ type: "text", text: "original user prompt" }],
      claudeToolResponses: [
        { toolUseId: "1", name: "Read", type: "tool_result" },
      ],
      claudeResponseBlocks: [
        { type: "tool_use", id: "2", name: "Edit", input: {} },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("marks claude user turn as callback when the model calls a tool", () => {
    const record = {
      claudeRequestBlocks: [{ type: "text", text: "fix the bug" }],
      claudeToolResponses: [],
      claudeResponseBlocks: [
        { type: "tool_use", id: "1", name: "Read", input: {} },
      ],
    };
    expect(parseInteractionType(record)).toBe("callback");
  });

  it("detects claude single turn from user input plus pure text output", () => {
    const record = {
      claudeRequestBlocks: [{ type: "text", text: "写一首诗" }],
      claudeToolResponses: [],
      claudeResponseBlocks: [{ type: "text", content: "春风拂柳岸" }],
    };
    expect(parseInteractionType(record)).toBe("single_turn");
  });

  it("detects responses single turn from request blocks plus output_text", () => {
    const record = {
      responsesRequestBlocks: [
        { type: "input_text", text: "讲个笑话", role: "user" },
      ],
      responsesToolResponses: [],
      responsesResponseBlocks: [
        { type: "output_text", content: "为什么程序员分不清万圣节和圣诞节……" },
      ],
    };
    expect(parseInteractionType(record)).toBe("single_turn");
  });

  it("detects bamboo single turn from request blocks plus response text", () => {
    const record = {
      bambooRequestBlocks: [{ text: "介绍一下你自己" }],
      bambooToolResponses: [],
      bambooResponseBlocks: [{ type: "text", text: "我是一个 AI 助手" }],
    };
    expect(parseInteractionType(record)).toBe("single_turn");
  });

  it("detects single turn from flattened prompt items plus completion", () => {
    const record = {
      prompt: {
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "input_text", text: "讲个笑话" }],
          },
        ],
      },
      completion: "为什么程序员分不清万圣节和圣诞节……",
    };
    expect(parseInteractionType(record)).toBe("single_turn");
  });

  it("keeps flattened prompt items as input when completion is missing", () => {
    const record = {
      prompt: {
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "input_text", text: "讲个笑话" }],
          },
        ],
      },
    };
    expect(parseInteractionType(record)).toBe("input");
  });

  it("detects single turn from legacy prompt and completion strings", () => {
    const record = {
      prompt: "什么是量子纠缠？",
      completion: "量子纠缠是……",
    };
    expect(parseInteractionType(record)).toBe("single_turn");
  });
});
