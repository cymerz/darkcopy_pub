import Link from 'next/link';

export function Footer() {
  const currentYear = new Date().getFullYear();

  return (
    <footer className="border-t border-zinc-200/80 dark:border-zinc-800/60 bg-zinc-50/30 dark:bg-zinc-950/20 backdrop-blur-md mt-auto">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 py-8 sm:py-10">
        <div className="flex flex-col md:flex-row items-center justify-between gap-6">
          {/* Left: Branding & Disclaimer */}
          <div className="flex flex-col items-center md:items-start text-center md:text-left gap-2 max-w-md">
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold tracking-wider text-gray-900 dark:text-gray-100 uppercase">
                Dark<span className="text-accent">Copy</span>
              </span>
              <span className="h-1.5 w-1.5 rounded-full bg-accent animate-pulse" />
            </div>
            <p className="text-xs text-gray-400 dark:text-gray-500 leading-relaxed">
              DarkCopy is an anonymous content sharing service. We are not responsible for any content uploaded by users. Illegal content may be reported for removal.
            </p>
            <p className="text-[10px] text-gray-400 dark:text-gray-600">
              &copy; {currentYear} DarkCopy. Hak Cipta Dilindungi.
            </p>
          </div>

          {/* Right: Navigation Links */}
          <nav className="flex flex-wrap justify-center items-center gap-x-6 gap-y-2 text-xs font-medium text-gray-500 dark:text-gray-400">
            <Link
              href="https://github.com/cymerz/darkcopy_pub"
              target="_blank"
              rel="noopener noreferrer"
              className="transition-colors hover:text-accent focus:outline-none"
            >
              GitHub
            </Link>
            <span aria-hidden="true" className="hidden sm:inline text-gray-300 dark:text-zinc-800">
              &middot;
            </span>
            <Link
              href="/new"
              className="transition-colors hover:text-accent focus:outline-none"
            >
              Buat Paste
            </Link>
            <span aria-hidden="true" className="hidden sm:inline text-gray-300 dark:text-zinc-800">
              &middot;
            </span>
            <Link
              href="/upload"
              className="transition-colors hover:text-accent focus:outline-none"
            >
              Unggah File
            </Link>
            <span aria-hidden="true" className="hidden sm:inline text-gray-300 dark:text-zinc-800">
              &middot;
            </span>
            <Link
              href="/admin"
              className="transition-colors hover:text-accent focus:outline-none"
            >
              Admin Panel
            </Link>
          </nav>
        </div>
      </div>
    </footer>
  );
}

export default Footer;
