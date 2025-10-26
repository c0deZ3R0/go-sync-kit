# Security Policy

## Supported Versions

We actively support the latest minor version with security updates. Older versions may receive critical fixes on a case-by-case basis.

| Version | Supported          |
| ------- | ------------------ |
| 0.24.x  | :white_check_mark: |
| < 0.24  | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

We take security seriously and appreciate responsible disclosure. If you discover a security issue, please report it privately using one of these methods:

### Preferred: GitHub Security Advisories

1. Navigate to the [Security tab](https://github.com/c0deZ3R0/go-sync-kit/security)
2. Click "Report a vulnerability"
3. Provide details about the vulnerability:
   - Description of the issue
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### Alternative: Private Email

If you prefer email or GitHub Security Advisories are unavailable, contact the maintainers privately through the email listed in the repository maintainer profile.

## What to Include

When reporting, please include:

- **Type of vulnerability** (e.g., SQL injection, XSS, auth bypass)
- **Affected component(s)** (e.g., storage/postgres, transport/httptransport)
- **Version(s) affected**
- **Steps to reproduce** (minimal code snippet if possible)
- **Potential impact** and attack scenarios
- **Suggested remediation** (optional)

## Response Timeline

- **Initial response**: Within 48 hours
- **Status update**: Within 7 days
- **Fix timeline**: Depends on severity
  - Critical: Within 7 days
  - High: Within 14 days
  - Medium/Low: Next minor release

## What NOT to Do

- ❌ Do not open public GitHub issues for vulnerabilities
- ❌ Do not disclose details publicly before a fix is released
- ❌ Do not exploit the vulnerability beyond proof-of-concept verification

## Security Update Process

1. We confirm and assess the vulnerability
2. We develop a fix in a private repository or branch
3. We coordinate disclosure timing with the reporter
4. We release a patched version and publish a security advisory
5. We credit the reporter (unless they prefer anonymity)

## Security Best Practices

When using go-sync-kit in production:

- **Keep dependencies updated**: Run `go get -u` regularly and monitor for security advisories
- **Use authentication**: Enable auth middleware for HTTP transports (Bearer/HMAC)
- **Validate inputs**: Sanitize event data and metadata before storage
- **Limit exposure**: Run services behind firewalls or API gateways
- **Monitor logs**: Enable structured logging and watch for anomalies
- **Review configurations**: Ensure stores (SQLite, PostgreSQL) use secure connection settings

See [docs/best-practices.md](docs/best-practices.md) for production deployment guidance.

## Disclosure Policy

We follow coordinated disclosure:

- We work with reporters to understand and fix issues before public disclosure
- We aim to release fixes before or simultaneously with public disclosure
- We credit reporters in release notes and security advisories (with permission)

## Contact

For security concerns, use GitHub Security Advisories or contact maintainers privately. For general questions, use GitHub Discussions or Issues.

---

**Thank you for helping keep go-sync-kit and its users safe!**
