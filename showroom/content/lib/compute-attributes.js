'use strict'

/**
 * Antora extension that derives computed attributes from known base attributes.
 *
 * Derived attributes:
 *   openshift_cluster_ingress_domain  — extracted from openshift_cluster_console_url
 *                                       e.g. "apps.cluster.example.com"
 *   perses_url                        — Perses UI route built from the ingress domain
 *
 * These are only set when the source attribute is a real URL (not an unreplaced
 * %placeholder% token), so local dev builds that stub attributes are unaffected.
 * If the deployer has already resolved the attribute the computed value is skipped.
 */
module.exports.register = function () {
  this.once('contentClassified', ({ contentCatalog }) => {
    contentCatalog.getComponents().forEach((component) => {
      component.versions.forEach((version) => {
        const attrs = (version.asciidoc && version.asciidoc.attributes) || {}

        const consoleUrl = attrs['openshift_cluster_console_url']
        // Only proceed when we have a real URL, not an unresolved %placeholder%
        if (!consoleUrl || consoleUrl.includes('%')) return

        // Derive the apps domain from the console URL.
        // Console URL format: https://console-openshift-console.apps.<cluster>.<base-domain>
        //                                                         ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        const match = consoleUrl.match(/https?:\/\/[^.]+\.(.+)$/)
        if (!match) return
        const appsDomain = match[1] // e.g. "apps.cluster-xyz.example.com"

        // Set openshift_cluster_ingress_domain unless already resolved
        if (!attrs['openshift_cluster_ingress_domain'] || attrs['openshift_cluster_ingress_domain'].includes('%')) {
          version.asciidoc.attributes['openshift_cluster_ingress_domain'] = appsDomain
        }

        // Set perses_url unless already resolved
        if (!attrs['perses_url'] || attrs['perses_url'].includes('%')) {
          version.asciidoc.attributes['perses_url'] = `https://perses-dev-perses-dev.${appsDomain}`
        }
      })
    })
  })
}
