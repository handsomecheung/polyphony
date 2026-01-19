let wasmInstance = null;

async function initWasm() {
  try {
    const response = await fetch("/wasm/main.wasm");
    const bytes = await response.arrayBuffer();
    const go = new Go();
    const result = await WebAssembly.instantiate(bytes, go.importObject);
    wasmInstance = result.instance;
    go.run(result.instance);

    setFont();
  } catch (err) {
    console.error("Failed to load WebAssembly:", err);
  }
}

checkWasmStatus = () => {
  if (!wasmInstance) {
    throw new Error("WebAssembly module is not initialized yet. Please wait for initialization to complete.");
  }
};

setFont = () => {
  checkWasmStatus();

  const defaultFont = "Yu-Gothic-UI";
  const font = typeof window !== "undefined" && window.localStorage ? window.localStorage.getItem("font") || defaultFont : defaultFont;

  try {
    const err = window.main.setFont(font);
    console.log("set font, error: ", err);
    return err;
  } catch (err) {
    console.error("Error in setFont:", err);
    throw new Error(`failed to set font: ${err.message}`);
  }
};

document.addEventListener("DOMContentLoaded", async function () {
  await initWasm();
});
