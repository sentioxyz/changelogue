"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

function LoginContent() {
  const params = useSearchParams();
  const error = params.get("error");

  const errorMessages: Record<string, string> = {
    unauthorized: "Your GitHub account is not authorized to access this application.",
    invalid_state: "Login session expired. Please try again.",
    missing_code: "GitHub did not return an authorization code.",
    token_exchange: "Failed to authenticate with GitHub. Please try again.",
    user_fetch: "Failed to fetch your GitHub profile. Please try again.",
    server_error: "An unexpected error occurred. Please try again.",
  };

  return (
    <div
      className="flex min-h-screen items-center justify-center"
      style={{ backgroundColor: "#f8f8f6" }}
    >
      <div className="w-full max-w-sm space-y-6 text-center">
        <div className="flex items-center justify-center gap-2">
          <img src="/logo.svg" alt="" className="h-8 w-8" />
          <span
            className="text-xl italic text-[#16181c]"
            style={{ fontFamily: "var(--font-fraunces)" }}
          >
            Changelogue
          </span>
        </div>

        <p className="text-sm text-[#6b7280]">
          Sign in to access your release intelligence dashboard.
        </p>

        {error && (
          <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {errorMessages[error] || "An error occurred. Please try again."}
          </div>
        )}

        <a
          href="/auth/github"
          className="inline-flex w-full items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-medium text-white transition-colors hover:opacity-90"
          style={{ backgroundColor: "#16181c" }}
        >
          <svg className="h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
          </svg>
          Sign in with GitHub
        </a>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginContent />
    </Suspense>
  );
}
