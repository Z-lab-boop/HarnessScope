import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

// Port zero lets the OS allocate a listener without a probe/bind race. The
// private IPC channel confirms readiness belongs to this exact child process.
export function startCaptureServer({ root = "../../dist/site", port = 0 } = {}) {
  const server = spawn(process.execPath, ["site/test-server.mjs", root, String(port)], {
    cwd: fileURLToPath(new URL("../", import.meta.url)),
    stdio: ["ignore", "ignore", "pipe", "ipc"],
  });
  return new Promise((resolve, reject) => {
    let diagnostics = "";
    server.stderr.on("data", chunk => { diagnostics += chunk.toString(); });
    const cleanup = () => {
      clearTimeout(timeout);
      server.off("error", onError);
      server.off("exit", onExit);
      server.off("message", onMessage);
    };
    const fail = error => {
      cleanup();
      server.kill("SIGTERM");
      reject(error);
    };
    const onError = error => fail(new Error(`capture server failed to start: ${error.message}`));
    const onExit = (code, signal) => fail(new Error(`capture server exited before listening (${code ?? signal}): ${diagnostics}`));
    const onMessage = message => {
      if (message?.type !== "listening" || message.pid !== server.pid || message.host !== "127.0.0.1" || !Number.isInteger(message.port) || message.port < 1 || message.port > 65535) {
        fail(new Error("capture server sent an invalid listening handshake"));
        return;
      }
      cleanup();
      resolve({ server, base: `http://127.0.0.1:${message.port}` });
    };
    const timeout = setTimeout(() => fail(new Error(`capture server did not confirm listening: ${diagnostics}`)), 5000);
    server.once("error", onError);
    server.once("exit", onExit);
    server.on("message", onMessage);
  });
}
