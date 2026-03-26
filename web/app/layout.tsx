import type { Metadata } from "next";
import { DM_Sans, Raleway } from "next/font/google";
import { AuthProvider } from "@/lib/auth/context";
import { Providers } from "@/components/providers";
import { LayoutShell } from "@/components/layout/layout-shell";
import "./globals.css";

const raleway = Raleway({
  variable: "--font-raleway",
  subsets: ["latin"],
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
      <body className={`${raleway.variable} ${dmSans.variable} antialiased`}>
        <Providers>
          <AuthProvider>
            <LayoutShell>{children}</LayoutShell>
          </AuthProvider>
        </Providers>
      </body>
    </html>
  );
}
