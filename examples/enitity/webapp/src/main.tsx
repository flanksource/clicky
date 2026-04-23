import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter } from "react-router-dom";
import { ThemeProvider, DensityProvider } from "@flanksource/clicky-ui";
// Register the <iconify-icon> web component so clicky-ui's <Icon> glyphs
// render (it loads icons on-demand from the Iconify CDN at runtime).
import "iconify-icon";
import "@flanksource/clicky-ui/styles.css";
import "./index.css";
import { App } from "./App";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 0, staleTime: 30_000 } },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider>
      <DensityProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </QueryClientProvider>
      </DensityProvider>
    </ThemeProvider>
  </React.StrictMode>,
);
