import { describe, expect, it } from "vitest";
import { buildArgs, findTool, tools } from "./tools";

describe("buildArgs", () => {
  it("puts subcommand and positionals before flags", () => {
    const t = findTool("nodepool-show")!;
    expect(buildArgs(t, { name: "np1", cluster: "c", resourceGroup: "rg" })).toEqual([
      "show", "np1", "--resource-group", "rg", "--cluster", "c",
    ]);
  });

  it("maps booleans, numbers and skips empties", () => {
    const t = findTool("restarts")!;
    expect(buildArgs(t, { all: true, min: 3, namespace: " " })).toEqual(["--all-namespaces", "--min", "3"]);
  });

  it("requires required fields", () => {
    expect(() => buildArgs(findTool("ping")!, { pod: "p" })).toThrow(/Destination is required/);
  });

  it("rejects flag injection and bad numbers", () => {
    expect(() => buildArgs(findTool("count")!, { resource: "--token=x" })).toThrow(/start with/);
    expect(() => buildArgs(findTool("ping")!, { pod: "p", destination: "1.1.1.1", count: "4; ls" })).toThrow(/whole number/);
  });

  it("splits multi values and adds --yes for confirmed deletes", () => {
    expect(buildArgs(findTool("nodepool-taint")!, { name: "p", taints: "a=b:NoSchedule  old-" })).toEqual(["taint", "p", "a=b:NoSchedule", "old-"]);
    expect(buildArgs(findTool("nodepool-delete")!, { name: "p" })).toEqual(["delete", "p", "--yes"]);
  });

  it("has unique ids", () => {
    expect(new Set(tools.map((t) => t.id)).size).toBe(tools.length);
  });
});
