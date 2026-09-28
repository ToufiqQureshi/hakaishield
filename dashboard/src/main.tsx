import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";

// Set initial theme class
const savedTheme = localStorage.getItem('hakaishield-theme') || 'dark';
document.documentElement.classList.add(savedTheme);

const app = import.meta.env.MODE === 'marketing'
  ? import('./MarketingApp.tsx')
  : import('./App.tsx');

app.then(({ default: App }) => {
  ReactDOM.createRoot(document.getElementById("root")!).render(<App />);
});
