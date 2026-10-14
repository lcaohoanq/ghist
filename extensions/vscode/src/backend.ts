import { spawn, type ChildProcess } from "node:child_process";
import type { Request, Response } from "./protocol";

export class BackendError extends Error {
  constructor(readonly code: string, message: string) {
    super(message);
  }
}

// No shell or terminal is involved. Each request owns its process, so cancelling
// one file's history cannot interrupt snapshots already open in another editor.
export class Backend {
  private readonly children = new Set<ChildProcess>();
  private disposed = false;

  constructor(private readonly executable: () => string) {}

  request<T>(request: Request, signal?: AbortSignal): Promise<T> {
    if (this.disposed || signal?.aborted) {
      return Promise.reject(new BackendError("cancelled", "Request cancelled."));
    }
    return new Promise<T>((resolve, reject) => {
      const child = spawn(this.executable(), [], { stdio: "pipe", windowsHide: true });
      this.children.add(child);
      const chunks: Buffer[] = [];
      let bytes = 0;
      let stderr = "";
      let failure: Error | undefined;
      let killTimer: NodeJS.Timeout | undefined;
      const stop = (error: Error) => {
        if (failure) { return; }
        failure = error;
        child.kill();
        killTimer = setTimeout(() => child.kill("SIGKILL"), 2000);
        killTimer.unref();
      };
      const cancel = () => stop(new BackendError("cancelled", "Request cancelled."));
      const timer = setTimeout(() => stop(new BackendError("timeout", "Git took too long. Try again or select a smaller history.")), 120_000);
      signal?.addEventListener("abort", cancel, { once: true });
      child.stdout!.on("data", (chunk: Buffer) => {
        bytes += chunk.length;
        if (bytes > 64 * 1024 * 1024) {
          stop(new BackendError("output_limit", "History exceeds the 64 MiB response limit."));
        } else if (!failure) {
          chunks.push(chunk);
        }
      });
      child.stderr!.on("data", (chunk: Buffer) => { stderr = (stderr + chunk.toString()).slice(-8192); });
      child.on("error", error => {
        failure = new BackendError("start_failed", `Cannot start ghist-backend: ${error.message}. Reinstall the extension or check ghist.backendPath.`);
      });
      child.stdin!.on("error", error => { failure ??= error; });
      child.on("close", code => {
        clearTimeout(timer);
        if (killTimer) { clearTimeout(killTimer); }
        signal?.removeEventListener("abort", cancel);
        this.children.delete(child);
        if (failure) { reject(failure); return; }
        if (code !== 0) {
          reject(new BackendError("process_failed", stderr.trim() || `ghist-backend exited with code ${code}.`));
          return;
        }
        try {
          const response = JSON.parse(Buffer.concat(chunks).toString("utf8")) as Response<T>;
          if (response.protocolVersion !== 1) {
            throw new Error("Unsupported backend protocol. Reinstall the extension.");
          }
          if (response.error) { throw new BackendError(response.error.code, response.error.message); }
          if (response.result === undefined) { throw new Error("Missing backend result."); }
          resolve(response.result);
        } catch (error) {
          reject(error);
        }
      });
      child.stdin!.end(JSON.stringify({ protocolVersion: 1, ...request }));
      // Cover cancellation between the initial check and listener registration.
      if (signal?.aborted) { cancel(); }
    });
  }

  dispose(): void {
    this.disposed = true;
    for (const child of this.children) { child.kill(); }
  }
}
