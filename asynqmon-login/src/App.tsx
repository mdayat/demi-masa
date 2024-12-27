import { Toaster } from "@components/solidui/Toast";
import type { ParentComponent } from "solid-js";

const App: ParentComponent = (props) => {
  return (
    <>
      <Toaster />
      {props.children}
    </>
  );
};

export default App;
