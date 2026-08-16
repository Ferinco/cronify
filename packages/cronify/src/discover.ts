import { existsSync, readFileSync, readdirSync } from "node:fs";
import { extname, join } from "node:path";

import ts from "typescript";

import { DEFAULT_MAX_ATTEMPTS, DEFAULT_TIMEOUT_SECONDS, type ManifestJobEntry } from "./types.js";

export interface DiscoveredJob extends ManifestJobEntry {
  sourceFile: string;
}

export class DiscoveryError extends Error {}

type Literal = string | number | boolean | null | undefined;

function walk(dir: string): string[] {
  let results: string[] = [];
  if (!existsSync(dir)) return results;
  const entries = readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      results = results.concat(walk(full));
    } else if (entry.isFile() && extname(entry.name) === ".ts" && !entry.name.endsWith(".d.ts")) {
      results.push(full);
    }
  }
  return results;
}

function literalToValue(node: ts.Expression): Literal {
  if (ts.isStringLiteralLike(node)) return node.text;
  if (ts.isNumericLiteral(node)) return Number(node.text);
  if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
  if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
  if (node.kind === ts.SyntaxKind.NullKeyword) return null;
  return undefined;
}

function findDefaultExportedDefineJobCall(sourceFile: ts.SourceFile): ts.CallExpression | null {
  for (const statement of sourceFile.statements) {
    if (!ts.isExportAssignment(statement) || statement.isExportEquals) continue;
    const expr = statement.expression;
    if (
      ts.isCallExpression(expr) &&
      ts.isIdentifier(expr.expression) &&
      expr.expression.text === "defineJob"
    ) {
      return expr;
    }
  }
  return null;
}

function hasNonDefaultDefineJobCall(sourceFile: ts.SourceFile): boolean {
  let found = false;
  function visit(node: ts.Node) {
    if (found) return;
    if (
      ts.isCallExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === "defineJob"
    ) {
      found = true;
      return;
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return found;
}

export function discoverJobs(cronDir: string): DiscoveredJob[] {
  const files = walk(cronDir);
  const jobs: DiscoveredJob[] = [];
  const seenIds = new Map<string, string>();

  for (const file of files) {
    const text = readFileSync(file, "utf8");
    const sourceFile = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);

    const call = findDefaultExportedDefineJobCall(sourceFile);
    if (!call) {
      if (hasNonDefaultDefineJobCall(sourceFile)) {
        throw new DiscoveryError(
          `${file}: defineJob() must be the default export, e.g. ` +
            `"export default defineJob({ ... })". cronify only discovers that exact shape.`,
        );
      }
      continue; // not a job file (e.g. a shared helper living under cron/) — skip
    }

    const arg = call.arguments[0];
    if (!arg || !ts.isObjectLiteralExpression(arg)) {
      throw new DiscoveryError(`${file}: defineJob() must be called with a single object literal argument.`);
    }

    const props: Record<string, Literal> = {};
    let hasHandler = false;
    for (const prop of arg.properties) {
      if (!ts.isPropertyAssignment(prop) || !ts.isIdentifier(prop.name)) {
        continue; // shorthand/spread/computed props fall through to the missing-field checks below
      }
      const key = prop.name.text;
      if (key === "handler") {
        hasHandler = true;
        continue;
      }
      const value = literalToValue(prop.initializer);
      if (value === undefined) {
        throw new DiscoveryError(
          `${file}: defineJob "${key}" must be a static literal (string/number/boolean), not a ` +
            `computed expression. cronify reads job metadata without executing your code.`,
        );
      }
      props[key] = value;
    }

    if (typeof props.id !== "string" || !props.id) {
      throw new DiscoveryError(`${file}: defineJob() is missing a string "id" literal.`);
    }
    if (typeof props.schedule !== "string" || !props.schedule) {
      throw new DiscoveryError(`${file}: defineJob() is missing a string "schedule" literal.`);
    }
    if (!hasHandler) {
      throw new DiscoveryError(`${file}: defineJob() is missing a "handler".`);
    }

    const id = props.id;
    if (seenIds.has(id)) {
      throw new DiscoveryError(`Duplicate job id "${id}" in ${file} and ${seenIds.get(id)}. Job ids must be unique.`);
    }
    seenIds.set(id, file);

    jobs.push({
      id,
      schedule: props.schedule,
      route: `/api/cron/${id}`,
      enabled: typeof props.enabled === "boolean" ? props.enabled : true,
      timeoutSeconds: typeof props.timeoutSeconds === "number" ? props.timeoutSeconds : DEFAULT_TIMEOUT_SECONDS,
      maxAttempts: typeof props.maxAttempts === "number" ? props.maxAttempts : DEFAULT_MAX_ATTEMPTS,
      description: typeof props.description === "string" ? props.description : null,
      sourceFile: file,
    });
  }

  return jobs.sort((a, b) => a.id.localeCompare(b.id));
}
