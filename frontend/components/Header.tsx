'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { ThemeToggle } from '@/components/ThemeToggle';

interface NavLink {
  href: string;
  label: string;
}

const NAV_LINKS: NavLink[] = [
  { href: '/', label: 'Beranda' },
  { href: '/new', label: 'Buat Paste' },
  { href: '/upload', label: 'Unggah File' },
];

function isActivePath(pathname: string | null, href: string): boolean {
  if (!pathname) return false;
  if (href === '/') {
    return pathname === '/';
  }
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function Header() {
  const pathname = usePathname();
  const [isMenuOpen, setIsMenuOpen] = useState(false);

  const closeMenu = () => setIsMenuOpen(false);
  const toggleMenu = () => setIsMenuOpen((open) => !open);

  return (
    <header className="sticky top-0 z-50 bg-white/80 dark:bg-dark-800/80 backdrop-blur-md border-b border-gray-200 dark:border-dark-700">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="relative flex items-center justify-between h-16">
          {/* Branding */}
          <Link
            href="/"
            onClick={closeMenu}
            className="flex min-h-[44px] items-center gap-2.5 group"
            aria-label="DarkCopy beranda"
          >
            <span
              className="flex items-center justify-center w-9 h-9 rounded-lg bg-accent text-white shadow-sm shadow-accent/30 group-hover:bg-accent-hover transition-colors"
              aria-hidden="true"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="w-5 h-5"
              >
                <path d="M13 2 4.5 13.5h6L9 22l9.5-12.5h-6L13 2z" />
              </svg>
            </span>
            <span className="font-mono font-bold text-lg tracking-tight text-gray-900 dark:text-gray-100 group-hover:text-black dark:group-hover:text-white transition-colors">
              DarkCopy
            </span>
          </Link>

          {/* Desktop Navigation — absolutely centered so it stays mid-header
              regardless of the logo / right-side control widths. */}
          <nav
            className="hidden md:flex items-center gap-1 absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2"
            aria-label="Navigasi utama"
          >
            {NAV_LINKS.map((link) => {
              const active = isActivePath(pathname, link.href);
              return (
                <Link
                  key={link.href}
                  href={link.href}
                  aria-current={active ? 'page' : undefined}
                  className={`relative inline-flex min-h-[44px] items-center px-4 py-2.5 rounded-md text-sm font-medium transition-colors ${
                    active
                      ? 'text-accent dark:text-accent-hover'
                      : 'text-gray-500 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white'
                  }`}
                >
                  {link.label}
                  {active && (
                    <span
                      aria-hidden="true"
                      className="absolute left-4 right-4 -bottom-px h-0.5 bg-accent dark:bg-accent-hover rounded-full"
                    />
                  )}
                </Link>
              );
            })}
          </nav>

          {/* Right-side controls: theme toggle + mobile menu button */}
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <button
              type="button"
              onClick={toggleMenu}
              aria-expanded={isMenuOpen}
              aria-controls="mobile-menu"
              aria-label={isMenuOpen ? 'Tutup menu' : 'Buka menu'}
              className="md:hidden inline-flex items-center justify-center w-11 h-11 rounded-md text-gray-500 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-dark-700/60 transition-colors"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                strokeLinecap="round"
                strokeLinejoin="round"
                className="w-6 h-6"
                aria-hidden="true"
              >
                {isMenuOpen ? (
                  <>
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </>
                ) : (
                  <>
                    <line x1="4" y1="7" x2="20" y2="7" />
                    <line x1="4" y1="12" x2="20" y2="12" />
                    <line x1="4" y1="17" x2="20" y2="17" />
                  </>
                )}
              </svg>
            </button>
          </div>
        </div>
      </div>

      {/* Mobile menu drawer */}
      <div
        id="mobile-menu"
        className={`md:hidden absolute top-full left-0 right-0 w-full overflow-hidden border-b border-gray-200 dark:border-dark-700 bg-white/95 dark:bg-dark-800/95 backdrop-blur-lg shadow-xl transition-all duration-200 ease-out origin-top ${
          isMenuOpen
            ? 'opacity-100 translate-y-0 scale-y-100 pointer-events-auto'
            : 'opacity-0 -translate-y-2 scale-y-95 pointer-events-none'
        }`}
      >
        <nav
          className="flex flex-col px-4 sm:px-6 py-4 gap-1.5"
          aria-label="Navigasi mobile"
        >
          {NAV_LINKS.map((link) => {
            const active = isActivePath(pathname, link.href);
            return (
              <Link
                key={link.href}
                href={link.href}
                onClick={closeMenu}
                aria-current={active ? 'page' : undefined}
                className={`flex min-h-[44px] items-center px-4 py-2.5 rounded-md text-base font-semibold transition-all ${
                  active
                    ? 'text-accent dark:text-accent-hover bg-accent/5 dark:bg-dark-700/30'
                    : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-50 dark:hover:bg-dark-700/20'
                }`}
              >
                {link.label}
              </Link>
            );
          })}
        </nav>
      </div>
    </header>
  );
}

export default Header;
