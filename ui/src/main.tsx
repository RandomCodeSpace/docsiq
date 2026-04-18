import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles/globals.css";
import App from "./App";

if (import.meta.env.DEV) {
  import("axe-core").then((axe) => {
    axe.default.run().then((res) => {
      if (res.violations.length > 0) {
        console.warn("axe violations:", res.violations);
      }
    });
  });
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
