import { Google } from "@components/icons/Google";
import { Button } from "@components/solidui/Button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@components/solidui/Card";
import { showToast } from "@components/solidui/Toast";
import { GoogleAuthProvider, signInWithPopup } from "@firebase/auth";
import { auth } from "@libs/firebase";

const provider = new GoogleAuthProvider();
const VITE_BACKEND_URL = import.meta.env.VITE_BACKEND_URL;

function Login() {
  const handleLogin = async () => {
    try {
      const userCredential = await signInWithPopup(auth, provider);

      const resp = await fetch(`${VITE_BACKEND_URL}/login`, {
        method: "POST",
        body: JSON.stringify({
          id_token: await userCredential.user.getIdToken(true),
          email: userCredential.user.email ?? "",
        }),
        headers: { "Content-Type": "application/json" },
      });

      if (resp.status === 200) {
        showToast({
          variant: "success",
          description:
            "Successfully login with Google account, you will be redirected in 3 seconds",
        });

        setTimeout(() => {
          window.location.replace(window.location.origin + "/monitoring");
        }, 3000);
      } else if (resp.status === 400) {
        throw new Error("invalid request body");
      } else if (resp.status === 401) {
        showToast({ variant: "error", description: "Invalid id token" });
      } else if (resp.status === 403) {
        showToast({
          variant: "error",
          description: "You are not authorized to login using this email",
        });
      } else if (resp.status >= 500) {
        showToast({ variant: "error", description: "Something went wrong" });
      } else {
        throw new Error(`unknown response status code ${resp.status}`);
      }
    } catch (error) {
      console.error(new Error("failed to login with Google", { cause: error }));
      showToast({
        variant: "error",
        description: "Failed to login with Google account",
      });
    }
  };

  return (
    <main class="min-h-screen flex items-center justify-center bg-gray-100 p-4">
      <Card class="w-full max-w-md">
        <CardHeader class="space-y-1">
          <CardTitle class="text-2xl font-bold text-center">
            Welcome back
          </CardTitle>
          <CardDescription class="text-center">
            Login to access Asynqmon
          </CardDescription>
        </CardHeader>

        <CardContent class="flex flex-col items-center">
          <div class="w-full max-w-xs mb-4">
            <img
              src="https://ec3q29jlfx8dke21.public.blob.vercel-storage.com/demi-masa-logo-hqkMxwY4lciC0StHA05IUeeWvw3jfq.png"
              alt="Demi Masa Logo"
              width={300}
              height={100}
              class="w-full h-auto"
            />
          </div>

          <Button
            onClick={handleLogin}
            class="bg-blue-600 hover:bg-blue-600/90 mx-auto flex items-center justify-between px-2.5 py-5 rounded-full"
          >
            <span class="rounded-full bg-white p-1">
              <Google />
            </span>
            <span class="text-white font-bold">Login dengan Google</span>
          </Button>
        </CardContent>
      </Card>
    </main>
  );
}

export default Login;
