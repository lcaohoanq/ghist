import { execFileSync } from "node:child_process";
import path from "node:path";
import { buildBackend, extensionRoot, hostTarget, targets } from "./build-backend.mjs";

const target = process.argv[2] || hostTarget;
if (!targets[target]) { throw new Error(`Unsupported target: ${target}`); }
for (const config of ["tsconfig.json", "tsconfig.webview.json"]) {
  execFileSync(process.execPath, [path.join(extensionRoot, "node_modules/typescript/bin/tsc"), "-p", path.join(extensionRoot, config)], { stdio: "inherit" });
}
buildBackend(target);
execFileSync(process.execPath, [path.join(extensionRoot, "node_modules/@vscode/vsce/vsce"), "package", "--target", target, "--no-dependencies", "--skip-license"], { cwd: extensionRoot, stdio: "inherit" });
