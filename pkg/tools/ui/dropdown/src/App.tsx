import React, { useState } from "react";
import { useApp, useHostStyles } from "@modelcontextprotocol/ext-apps/react";

const appInfo = {
  name: "gke-ui-components-app",
  version: "1.0.0",
};

export default function App() {
  const [options, setOptions] = useState<string[]>([]);
  const [selected, setSelected] = useState<string>("");
  const [submitted, setSubmitted] = useState<boolean>(false);
  const [title, setTitle] = useState<string>("Select a GKE Resource:");

  const { app, isConnected } = useApp({
    appInfo,
    capabilities: {},
    onAppCreated: (app) => {
      // Reset state when new tool input is received (new interaction)
      app.ontoolinput = (params) => {
        const inputOptions = params.arguments?.options as string[];
        const inputTitle = params.arguments?.title as string;

        if (Array.isArray(inputOptions)) {
          setOptions(inputOptions);
        }
        if (inputTitle) {
          setTitle(inputTitle);
        }
      };
    },
  });

  useHostStyles(app);

  const handleSelect = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value;
    setSelected(value);
    setSubmitted(true);

    // Allow UI to update before sending message

    try {
      // Call the internal tool to submit the selection
      await app?.sendMessage({ role: "user", content: [{ type: "text", text: value }] })
    } catch (error) {
      console.error("Failed to send message:", error);
    }

  };

  if (!isConnected) {
    return (
      <div className="loading">
        Loading GKE Components...
      </div>
    );
  }

  return (
    <div className="container">
      <div className="input-group">
        <label htmlFor="resource-dropdown" className="label">
          {title}
        </label>
        <div className="select-wrapper">
          <select
            id="resource-dropdown"
            value={selected}
            onChange={handleSelect}
            className={`select ${submitted ? "submitted" : ""}`}
            disabled={submitted}
          >
            <option value="" disabled>-- Choose an option --</option>
            {options.map((opt) => (
              <option key={opt} value={opt}>
                {opt}
              </option>
            ))}
          </select>
          <div className="select-arrow" />
        </div>
      </div>
    </div>
  );
}
