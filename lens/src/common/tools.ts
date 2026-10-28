// The kubectl-plugins tools the extension can run, and how a form's values
// become a command line. The main process rebuilds the arguments from this
// table itself, so the renderer can only ever run a tool listed here.

export type FieldKind = "text" | "number" | "boolean";

export interface Field {
  name: string;
  label: string;
  kind: FieldKind;
  /** Long flag the value is passed with, e.g. "--namespace". Omit for a positional argument. */
  flag?: string;
  required?: boolean;
  placeholder?: string;
  /** Shown under "Advanced" instead of inline. */
  advanced?: boolean;
  /** Positional field holding several whitespace-separated values. */
  multi?: boolean;
}

export type Category = "Pods" | "Cluster" | "Network" | "AKS" | "AKS changes";

export interface Tool {
  id: string;
  title: string;
  category: Category;
  description: string;
  /** Executable name on PATH. */
  binary: string;
  /** Fixed arguments placed before the form's, e.g. a subcommand. */
  args?: string[];
  fields: Field[];
  /** Changes cluster or Azure resources; the UI asks for confirmation first. */
  mutating?: boolean;
  /** Pass --yes because the UI has already confirmed. */
  autoYes?: boolean;
  /** Seconds before the run is killed. */
  timeoutSeconds?: number;
}

export type Values = Record<string, string | number | boolean | undefined>;

const namespace: Field = { name: "namespace", label: "Namespace", kind: "text", flag: "--namespace", placeholder: "default (context namespace)" };
const allNamespaces: Field = { name: "all", label: "All namespaces", kind: "boolean", flag: "--all-namespaces" };
const selector: Field = { name: "selector", label: "Label selector", kind: "text", flag: "--selector", placeholder: "app=web" };
const podScope = [namespace, allNamespaces, selector];

const aksTarget: Field[] = [
  { name: "subscription", label: "Subscription ID", kind: "text", flag: "--subscription", advanced: true },
  { name: "resourceGroup", label: "Resource group", kind: "text", flag: "--resource-group", advanced: true },
  { name: "cluster", label: "AKS cluster name", kind: "text", flag: "--cluster", advanced: true },
];

const poolName: Field = { name: "name", label: "Node pool name", kind: "text", required: true };

export const tools: Tool[] = [
  { id: "unhealthy", title: "Unhealthy pods", category: "Pods", binary: "kubectl-unhealthy", description: "Pods that are not Ready, crash-looping, failed or terminating, with the reason.", fields: podScope },
  { id: "pending", title: "Pending pods", category: "Pods", binary: "kubectl-pending", description: "Pods stuck in Pending, with the scheduler's reason.", fields: podScope },
  {
    id: "restarts", title: "Restarts", category: "Pods", binary: "kubectl-restarts",
    description: "Pods ranked by container restarts, with the last termination reason.",
    fields: [...podScope,
      { name: "min", label: "Minimum restarts", kind: "number", flag: "--min", placeholder: "1" },
      { name: "limit", label: "Max rows", kind: "number", flag: "--limit", placeholder: "20 (0 for all)" }],
  },
  {
    id: "images", title: "Images", category: "Pods", binary: "kubectl-images",
    description: "Container images in use; flags :latest and untagged ones.",
    fields: [...podScope, { name: "latest", label: "Only :latest / untagged", kind: "boolean", flag: "--latest" }],
  },
  {
    id: "count", title: "Count resources", category: "Cluster", binary: "kubectl-count",
    description: "Count resources of any type, e.g. services or deployments.",
    fields: [{ name: "resource", label: "Resource type", kind: "text", required: true, placeholder: "services" }, namespace, allNamespaces, selector],
  },
  {
    id: "orphans", title: "Orphans", category: "Cluster", binary: "kubectl-orphans",
    description: "Unused ConfigMaps, Secrets, PVCs and Services with no endpoints.",
    fields: [namespace, allNamespaces, { name: "type", label: "Types", kind: "text", flag: "--type", placeholder: "configmaps,secrets,pvcs,services" }],
  },
  { id: "whoami", title: "Who am I", category: "Cluster", binary: "kubectl-whoami", description: "The current user, groups and context.", fields: [] },
  {
    id: "ping", title: "Ping from a pod", category: "Network", binary: "kubectl-ping",
    description: "Ping an IPv4 address or DNS name from inside a pod.",
    timeoutSeconds: 1200,
    fields: [
      { name: "pod", label: "Pod", kind: "text", required: true },
      { name: "destination", label: "Destination", kind: "text", required: true, placeholder: "8.8.8.8 or my-svc.ns.svc.cluster.local" },
      namespace,
      { name: "container", label: "Container", kind: "text", flag: "--container" },
      { name: "count", label: "Packets", kind: "number", flag: "--count", placeholder: "4" },
      { name: "size", label: "Payload size (bytes)", kind: "number", flag: "--size" },
      { name: "timeout", label: "Reply timeout (s)", kind: "number", flag: "--timeout", placeholder: "2" },
    ],
  },
  { id: "aks-show", title: "AKS cluster", category: "AKS", binary: "kubectl-aks", args: ["show"], description: "Summary of the AKS cluster behind this context.", timeoutSeconds: 180, fields: aksTarget },
  { id: "aks-describe", title: "AKS cluster details", category: "AKS", binary: "kubectl-aks", args: ["describe"], description: "Full AKS cluster properties and upgrade profile.", timeoutSeconds: 180, fields: aksTarget },
  { id: "aks-list", title: "AKS clusters", category: "AKS", binary: "kubectl-aks", args: ["list"], description: "AKS clusters in your Azure subscriptions.", timeoutSeconds: 180, fields: [aksTarget[0]] },
  { id: "nodepool-list", title: "Node pools", category: "AKS", binary: "kubectl-nodepool", args: ["list"], description: "Node pools of the AKS cluster.", timeoutSeconds: 180, fields: aksTarget },
  { id: "nodepool-show", title: "Node pool", category: "AKS", binary: "kubectl-nodepool", args: ["show"], description: "Summary of one node pool.", timeoutSeconds: 180, fields: [poolName, ...aksTarget] },
  { id: "nodepool-describe", title: "Node pool details", category: "AKS", binary: "kubectl-nodepool", args: ["describe"], description: "Full node pool properties, node image and upgrade versions.", timeoutSeconds: 180, fields: [poolName, ...aksTarget] },
  {
    id: "aks-versions", title: "Kubernetes versions", category: "AKS", binary: "kubectl-aksversions",
    description: "Kubernetes versions available in a region.", timeoutSeconds: 180,
    fields: [
      { name: "location", label: "Region", kind: "text", flag: "--location", placeholder: "eastus (default: cluster's region)" },
      { name: "patches", label: "Show patch versions", kind: "boolean", flag: "--patches" },
      ...aksTarget,
    ],
  },
  { id: "aks-maintenance-list", title: "Maintenance windows", category: "AKS", binary: "kubectl-aksmaintenance", args: ["list"], description: "Planned maintenance configurations.", timeoutSeconds: 180, fields: aksTarget },
  {
    id: "aks-maintenance-show", title: "Maintenance window", category: "AKS", binary: "kubectl-aksmaintenance", args: ["show"],
    description: "One maintenance configuration (default, auto-upgrade or node-os).", timeoutSeconds: 180,
    fields: [{ name: "name", label: "Configuration", kind: "text", required: true, placeholder: "default | auto-upgrade | node-os" }, ...aksTarget],
  },
];

const op = 1800;
const mut = { category: "AKS changes" as Category, mutating: true, timeoutSeconds: op };
const version: Field = { name: "version", label: "Kubernetes version", kind: "text", flag: "--kubernetes-version", required: true, placeholder: "1.30.4" };
const noWait: Field = { name: "noWait", label: "Do not wait for completion", kind: "boolean", flag: "--no-wait" };
const flagText = (name: string, label: string, flag: string, placeholder?: string): Field => ({ name, label, kind: "text", flag, placeholder });
const flagNum = (name: string, label: string, flag: string, placeholder?: string): Field => ({ name, label, kind: "number", flag, placeholder });
const flagBool = (name: string, label: string, flag: string): Field => ({ name, label, kind: "boolean", flag });

tools.push(
  { id: "aks-upgrade", title: "Upgrade cluster", binary: "kubectl-aks", args: ["upgrade"], description: "Upgrade the AKS control plane Kubernetes version.", ...mut, autoYes: false, fields: [version, noWait, ...aksTarget] },
  { id: "aks-start", title: "Start cluster", binary: "kubectl-aks", args: ["start"], description: "Start a stopped AKS cluster.", ...mut, fields: [noWait, ...aksTarget] },
  { id: "aks-stop", title: "Stop cluster", binary: "kubectl-aks", args: ["stop"], description: "Stop the AKS cluster. Workloads go down.", ...mut, fields: [noWait, ...aksTarget] },
  { id: "aks-rotate-certs", title: "Rotate certificates", binary: "kubectl-aks", args: ["rotate-certs"], description: "Rotate the cluster certificates. The API server restarts.", ...mut, autoYes: true, fields: [noWait, ...aksTarget] },
  {
    id: "nodepool-add", title: "Add node pool", binary: "kubectl-nodepool", args: ["add"], description: "Create a node pool.", ...mut,
    fields: [
      poolName, flagText("vmSize", "VM size", "--vm-size", "Standard_DS2_v2"), flagNum("count", "Node count", "--count", "3"),
      flagText("mode", "Mode", "--mode", "User | System"), flagText("version", "Kubernetes version", "--kubernetes-version"),
      flagText("labels", "Labels", "--labels", "key=value,key2=value2"), flagText("taints", "Taints", "--taints", "key=value:NoSchedule"),
      flagBool("autoscale", "Enable cluster autoscaler", "--enable-cluster-autoscaler"), flagNum("minCount", "Autoscaler min", "--min-count"),
      flagNum("maxCount", "Autoscaler max", "--max-count"), flagText("maxSurge", "Max surge", "--max-surge", "33% or 2"),
      flagText("osType", "OS type", "--os-type", "Linux | Windows"), flagNum("maxPods", "Max pods per node", "--max-pods"),
      flagText("zones", "Availability zones", "--zones", "1,2,3"), flagText("subnet", "Subnet resource ID", "--vnet-subnet-id"),
      flagNum("osDisk", "OS disk size (GB)", "--os-disk-size"), noWait, ...aksTarget,
    ],
  },
  { id: "nodepool-delete", title: "Delete node pool", binary: "kubectl-nodepool", args: ["delete"], description: "Delete a node pool and its nodes.", ...mut, autoYes: true, fields: [poolName, noWait, ...aksTarget] },
  { id: "nodepool-scale", title: "Scale node pool", binary: "kubectl-nodepool", args: ["scale"], description: "Set the node count of a pool.", ...mut, fields: [poolName, { ...flagNum("count", "Node count", "--count"), required: true }, noWait, ...aksTarget] },
  { id: "nodepool-upgrade", title: "Upgrade node pool", binary: "kubectl-nodepool", args: ["upgrade"], description: "Upgrade the Kubernetes version of a pool.", ...mut, fields: [poolName, version, noWait, ...aksTarget] },
  { id: "nodepool-upgrade-node-image", title: "Upgrade node image", binary: "kubectl-nodepool", args: ["upgrade-node-image"], description: "Upgrade the node image of a pool.", ...mut, fields: [poolName, noWait, ...aksTarget] },
  { id: "nodepool-start", title: "Start node pool", binary: "kubectl-nodepool", args: ["start"], description: "Start a stopped pool.", ...mut, fields: [poolName, noWait, ...aksTarget] },
  { id: "nodepool-stop", title: "Stop node pool", binary: "kubectl-nodepool", args: ["stop"], description: "Stop a pool. Its workloads go down.", ...mut, fields: [poolName, noWait, ...aksTarget] },
  { id: "nodepool-cancel", title: "Cancel pool operation", binary: "kubectl-nodepool", args: ["cancel-operation"], description: "Cancel the latest operation on a pool.", ...mut, fields: [poolName, noWait, ...aksTarget] },
  {
    id: "nodepool-update", title: "Update node pool", binary: "kubectl-nodepool", args: ["update"], description: "Change autoscaler, labels, taints, surge or mode. Only filled fields are sent.", ...mut,
    fields: [
      poolName, flagBool("enableAs", "Enable cluster autoscaler", "--enable-cluster-autoscaler"), flagBool("disableAs", "Disable cluster autoscaler", "--disable-cluster-autoscaler"),
      flagNum("minCount", "Autoscaler min", "--min-count"), flagNum("maxCount", "Autoscaler max", "--max-count"),
      flagText("labels", "Replace labels", "--labels", "key=value,..."), flagText("taints", "Replace taints", "--taints", "key=value:Effect,..."),
      flagText("maxSurge", "Max surge", "--max-surge"), flagText("mode", "Mode", "--mode", "User | System"), noWait, ...aksTarget,
    ],
  },
  {
    id: "nodepool-taint", title: "Taint node pool", binary: "kubectl-nodepool", args: ["taint"], description: "Add taints, or remove with KEY[:EFFECT]-.", ...mut,
    fields: [poolName, { name: "taints", label: "Taints", kind: "text", multi: true, required: true, placeholder: "dedicated=gpu:NoSchedule  old-"  }, noWait, ...aksTarget],
  },
  {
    id: "nodepool-untaint", title: "Untaint node pool", binary: "kubectl-nodepool", args: ["untaint"], description: "Remove taints by KEY[:EFFECT].", ...mut,
    fields: [poolName, { name: "taints", label: "Taint keys", kind: "text", multi: true, required: true, placeholder: "dedicated:NoSchedule" }, noWait, ...aksTarget],
  },
  {
    id: "aks-maintenance-add", title: "Add maintenance window", binary: "kubectl-aksmaintenance", args: ["add"], description: "Create or replace a maintenance configuration.", ...mut,
    fields: [
      { name: "name", label: "Configuration", kind: "text", required: true, placeholder: "default | auto-upgrade | node-os" },
      { ...flagText("day", "Day of week", "--day", "Sunday"), required: true }, flagText("start", "Start time", "--start-time", "02:00"),
      flagNum("duration", "Duration (hours)", "--duration-hours", "4"), flagNum("interval", "Interval (weeks)", "--interval-weeks", "1"),
      flagText("utc", "UTC offset", "--utc-offset", "+02:00"), flagText("hours", "UTC hour slots (default config)", "--hours", "1,2,3"), ...aksTarget,
    ],
  },
  { id: "aks-maintenance-delete", title: "Delete maintenance window", binary: "kubectl-aksmaintenance", args: ["delete"], description: "Delete a maintenance configuration.", ...mut, fields: [{ name: "name", label: "Configuration", kind: "text", required: true }, ...aksTarget] },
  {
    id: "settags", title: "Set tags (settags)", category: "Cluster", mutating: true, autoYes: true, binary: "kubectl-settags", timeoutSeconds: 300,
    description: "Set tags on resources. Tick Dry run to preview the changes without applying them.",
    fields: [
      { name: "pairs", label: "NAME=TAG pairs", kind: "text", multi: true, placeholder: "web=v2 api=v3" },
      { name: "file", label: "File with NAME=TAG lines", kind: "text", flag: "--file" },
      flagBool("dry", "Dry run", "--dry-run"), allNamespaces, namespace,
    ],
  },
);

export const categories: Category[] = ["Pods", "Cluster", "Network", "AKS", "AKS changes"];

export function findTool(id: string): Tool | undefined {
  return tools.find((t) => t.id === id);
}

const maxValueLength = 512;

function textOf(field: Field, raw: Values[string]): string {
  if (raw === undefined || raw === null) return "";
  if (typeof raw === "boolean") throw new Error(`${field.label} must be text`);
  const s = String(raw).trim();
  if (s.length > maxValueLength) throw new Error(`${field.label} is too long`);
  if (/[\0\r\n]/.test(s)) throw new Error(`${field.label} contains invalid characters`);
  return s;
}

/**
 * Turns form values into the argument list for the tool's executable. Values are
 * passed as separate argv entries (never through a shell), and positional values
 * may not start with "-" so they cannot be read as flags.
 */
export function buildArgs(tool: Tool, values: Values): string[] {
  const args = [...(tool.args ?? [])];
  const flags: string[] = [];

  for (const f of tool.fields) {
    const raw = values[f.name];
    if (f.kind === "boolean") {
      if (raw === true) {
        if (!f.flag) throw new Error(`${f.label} cannot be a positional switch`);
        flags.push(f.flag);
      }
      continue;
    }
    const s = textOf(f, raw);
    if (s === "") {
      if (f.required) throw new Error(`${f.label} is required`);
      continue;
    }
    if (f.kind === "number" && !/^\d+$/.test(s)) throw new Error(`${f.label} must be a non-negative whole number`);
    if (f.multi) {
      for (const part of s.split(/\s+/)) {
        if (part.startsWith("-")) throw new Error(`${f.label} may not start with "-"`);
        args.push(part);
      }
    } else if (f.flag) flags.push(f.flag, s);
    else if (s.startsWith("-")) throw new Error(`${f.label} may not start with "-"`);
    else args.push(s);
  }
  if (tool.autoYes) flags.push("--yes");
  return [...args, ...flags];
}
