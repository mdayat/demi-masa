/* @refresh reload */
import "./index.css";

import { Route, Router } from "@solidjs/router";
import { render } from "solid-js/web";
import { lazy } from "solid-js";
import App from "./App";
import Login from "./pages/Login";

const NotFound = lazy(() =>
  import("@components/NotFound").then(({ NotFound }) => ({ default: NotFound }))
);

const root = document.getElementById("root") as HTMLElement;
if (import.meta.env.DEV && !(root instanceof HTMLElement)) {
  throw new Error(
    "Root element not found. Did you forget to add it to your index.html? Or maybe the id attribute got misspelled?"
  );
}

render(
  () => (
    <Router root={(props) => <App>{props.children}</App>}>
      <Route path="/" component={Login} />
      <Route path="**" component={NotFound} />
    </Router>
  ),
  root
);
