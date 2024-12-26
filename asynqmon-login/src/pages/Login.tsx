import { Google } from "@components/icons/Google";
import { Button } from "@components/solidui/Button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@components/solidui/Card";

function Login() {
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

          <Button class="bg-blue-600 hover:bg-blue-600/90 mx-auto flex items-center justify-between px-2.5 py-5 rounded-full">
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
