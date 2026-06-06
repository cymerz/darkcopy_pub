import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { Header } from '@/components/Header';
import { Footer } from '@/components/Footer';
import { NavigationProgress } from '@/components/NavigationProgress';
import './globals.css';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-inter',
});

export const metadata: Metadata = {
  title: 'DarkCopy',
  description: 'Platform berbagi teks dan file dengan tema gelap',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="id" className={`${inter.variable} dark overflow-x-hidden`} suppressHydrationWarning>
      <head>
        {/* Inline script to prevent FOUC: apply stored theme before paint */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('theme');if(t==='light'){document.documentElement.classList.remove('dark')}else if(t==='dark'||!t){document.documentElement.classList.add('dark')}}catch(e){}})()`,
          }}
        />
      </head>
      <body className="bg-white dark:bg-dark-900 text-gray-900 dark:text-gray-100 min-h-screen flex flex-col font-sans antialiased transition-colors relative overflow-x-hidden">
        {/* Decorative glowing gradient meshes: hidden on mobile to maximize scroll performance, active on desktop */}
        <div className="hidden md:block absolute inset-0 overflow-hidden pointer-events-none -z-10">
          <div className="absolute top-[-10%] left-[-20%] w-[600px] h-[600px] rounded-full bg-accent/10 dark:bg-accent/5 blur-[120px]" />
          <div className="absolute bottom-[20%] right-[-20%] w-[600px] h-[600px] rounded-full bg-purple-500/10 dark:bg-purple-500/5 blur-[120px]" />
        </div>

        <NavigationProgress />
        <Header />
        <main className="flex-1 w-full max-w-6xl mx-auto px-4 py-6 md:px-6 md:py-8 lg:px-8 lg:py-10 relative z-10">
          {children}
        </main>
        <Footer />
      </body>
    </html>
  );
}
