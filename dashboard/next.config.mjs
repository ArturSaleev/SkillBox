/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: "export",
  trailingSlash: true,
  images: { unoptimized: true },
  turbopack: { root: process.cwd() },
  agentRules: false,
};

export default nextConfig;
