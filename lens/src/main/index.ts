import { spawn } from "node:child_process";
import { homedir } from "node:os";
import { delimiter, join } from "node:path";
import { Main } from "@freelensapp/extensions";
import { buildArgs, findTool, type Values } from "../common/tools";

export interface RunRequest {
  clusterId: string;
  toolId: string;
  values: Values;
}

export interface RunResult {
  ok: boolean;
  output: string;
}

const maxOutputBytes = 1024 * 1024;

// GUI apps start with a minimal PATH; add where `make install` and Go put binaries.
function searchPath(): string {
  const extra = [
    join(homedir(), ".local", "bin"),
    join(homedir(), "go", "bin"),
    "/usr/local/bin",
    "/opt/homebrew/bin",
  ];
  if (process.env.GOBIN) extra.unshift(process.env.GOBIN);
  return [...extra, process.env.PATH ?? ""].filter(Boolean).join(delimiter);
}

export function run(req: RunRequest): Promise<RunResult> {
  const tool = findTool(String(req?.toolId));
  if (!tool) return Promise.resolve({ ok: false, output: `Unknown tool: ${req?.toolId}` });

  const cluster = Main.Catalog.getClusterById(String(req.clusterId));
  if (!cluster) return Promise.resolve({ ok: false, output: "Cluster not found." });

  let args: string[];
  try {
    args = [...buildArgs(tool, req.values ?? {}), "--kubeconfig", cluster.kubeConfigPath, "--context", cluster.contextName];
  } catch (e) {
    return Promise.resolve({ ok: false, output: (e as Error).message });
  }

  return new Promise((resolve) => {
    const child = spawn(tool.binary, args, {
      env: { ...process.env, PATH: searchPath(), NO_COLOR: "1" },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let out = "";
    let truncated = false;
    const append = (b: Buffer) => {
      if (out.length >= maxOutputBytes) {
        truncated = true;
        return;
      }
      out += b.toString("utf8");
    };
    child.stdout.on("data", append);
    child.stderr.on("data", append);

    const timer = setTimeout(() => {
      child.kill();
      out += `\n[timed out after ${tool.timeoutSeconds ?? 60}s]`;
    }, (tool.timeoutSeconds ?? 60) * 1000);

    child.on("error", (err: NodeJS.ErrnoException) => {
      clearTimeout(timer);
      const hint =
        err.code === "ENOENT"
          ? `${tool.binary} was not found on PATH. Install it with \`make install\` (see the kubectl-plugins README).`
          : err.message;
      resolve({ ok: false, output: hint });
    });
    child.on("close", (code) => {
      clearTimeout(timer);
      resolve({ ok: code === 0, output: out + (truncated ? "\n[output truncated]" : "") });
    });
  });
}

class PluginsIpc extends Main.Ipc {
  constructor(extension: Main.LensExtension) {
    super(extension);
    this.handle("run", (_event, req: RunRequest) => run(req));
  }
}

export default class KubectlPluginsMain extends Main.LensExtension {
  async onActivate() {
    PluginsIpc.createInstance(this);
  }
}
