import { lazy } from "solid-js";
import { Route, Router } from "@solidjs/router";
import Login from "./pages/Login";

const NotFound = lazy(() =>
  import("@components/NotFound").then(({ NotFound }) => ({ default: NotFound }))
);

function App() {
  return (
    <Router>
      <Route path="/login" component={Login} />
      <Route path="**" component={NotFound} />
    </Router>
  );
}

export default App;
