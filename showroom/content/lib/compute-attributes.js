'use strict'

/**
 * Antora extension that derives computed attributes from the ingress domain.
 *
 * When showroom-deployer substitutes %openshift_cluster_ingress_domain% before
 * the Antora build, this extension reconstructs all URL attributes and page-links
 * from the resolved domain so they are correct in the generated HTML.
 *
 * For GitHub Pages builds (where attributes still contain %placeholders%), the
 * guard condition prevents any execution — nookbag handles substitution client-side.
 */
module.exports.register = function () {
  this.once('contentClassified', ({ contentCatalog }) => {
    contentCatalog.getComponents().forEach((component) => {
      component.versions.forEach((version) => {
        const attrs = (version.asciidoc && version.asciidoc.attributes) || {}

        const domain = attrs['openshift_cluster_ingress_domain']
        // Only proceed when we have a real domain, not an unresolved %placeholder%
        if (!domain || domain.includes('%')) return

        const consoleUrl = `https://console-openshift-console.${domain}`
        const persesUrl = `https://perses.${domain}`
        const apiUrl = `https://api.${domain}:6443`

        version.asciidoc.attributes['openshift_cluster_console_url'] = consoleUrl
        version.asciidoc.attributes['openshift_console_url'] = consoleUrl
        version.asciidoc.attributes['perses_url'] = persesUrl
        version.asciidoc.attributes['openshift_api_url'] = apiUrl

        // Rebuild page-links with resolved URLs
        version.asciidoc.attributes['page-links'] = [
          { url: consoleUrl, text: 'OCP Console' },
          { url: `${consoleUrl}/terminal`, text: 'Web Terminal' },
          { url: 'https://redhat.com', text: 'Red Hat' },
        ]
      })
    })
  })
}
