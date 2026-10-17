import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { LiveApp } from "./live";
import { Gallery } from "./workspace";
import "./style.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30000 } },
});
const root = document.getElementById("root");
if (!root) throw new Error("Application root is missing");
createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        {window.location.pathname === "/gallery" ? <Gallery /> : <LiveApp />}
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
