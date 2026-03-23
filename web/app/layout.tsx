import type { Metadata } from "next";
import { Fraunces, DM_Sans } from "next/font/google";
import { AuthProvider } from "@/lib/auth/context";
import { Providers } from "@/components/providers";
import { LayoutShell } from "@/components/layout/layout-shell";
import "./globals.css";

const fraunces = Fraunces({
  variable: "--font-fraunces",
  subsets: ["latin"],
  axes: ["SOFT", "WONK"],
  display: "swap",
});

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "Changelogue",
  description: "Agent-driven release intelligence platform",
  icons: {
    icon: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${fraunces.variable} ${dmSans.variable} antialiased`}>
        <Providers>
          <AuthProvider>
            <LayoutShell>{children}</LayoutShell>
          </AuthProvider>
        </Providers>
      </body>
    </html>
  );
}
