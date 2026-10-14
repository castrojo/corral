# Security Policy

## Reporting a Vulnerability

The tuna-os project takes security seriously. If you discover a security vulnerability in Corral, please report it responsibly by emailing **security@tuna-os.org** with:

1. **Description**: What is the vulnerability?
2. **Affected components**: Which Corral components or versions are affected?
3. **Impact**: What could an attacker do?
4. **Reproduction steps**: How can the issue be reproduced (if applicable)?
5. **Proposed fix**: Any suggestions for remediation (optional)

**Please do not open a public GitHub issue for security vulnerabilities.** Public disclosure before a fix is available can put users at risk.

## Disclosure Timeline

We aim to:
- Acknowledge receipt of your report within 48 hours
- Provide an initial assessment within 5 days
- Release a fix or workaround within 30 days (for critical issues)
- Publish a security advisory once a fix is available

## Scope

### In Scope
- Authentication and authorization mechanisms (Tailnet trust model, RBAC)
- Credential and secret handling
- API security (TLS, certificate validation)
- Dependency vulnerabilities
- Injection attacks and input validation issues

### Out of Scope
- Social engineering or phishing
- Physical security issues
- Vulnerabilities in unpatched upstream dependencies (report directly to upstream)
- Issues in deployment configurations (Docker, Kubernetes) outside Corral's control

## Known Security Considerations

### Tailnet Trust Model
Corral relies on WireGuard Tailnet for transport security. **Network traffic is trusted once on the Tailnet.** Ensure your Tailnet is properly configured and access is restricted to trusted users.

### Ingress Configuration
When exposing Corral via Ingress or reverse proxy, ensure:
- TLS is enabled and properly configured
- Origin validation is enforced
- Authentication/authorization is in place before Corral

### Kubernetes RBAC
Review the default service account permissions and cluster roles. Grant minimal necessary permissions per your deployment model.

## Security Updates

Corral follows semantic versioning. Security patches are released as patch versions (e.g., 1.0.1) and are marked as security updates in release notes.

Subscribe to GitHub release notifications to stay informed of security updates.

## Additional Resources

- [GitHub Security Advisory Form](https://github.com/security/advisories/new)
- [CNCF Security Best Practices](https://www.cncf.io/blog/2019/08/06/open-source-security-best-practices/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)

---

For questions about this security policy, open a GitHub discussion or contact security@tuna-os.org.
