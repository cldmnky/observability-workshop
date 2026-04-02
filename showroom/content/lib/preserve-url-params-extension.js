'use strict'

// Patches Antora-generated redirect pages to preserve URL query parameters.
// Without this, navigating to the site root with ?KEY=VALUE parameters loses
// them during the JavaScript redirect to the start page.
module.exports.register = function () {
  this.on('beforePublish', ({ siteCatalog, contentCatalog }) => {
    ;[...siteCatalog.getFiles(), ...contentCatalog.getFiles()].forEach((file) => {
      if (file.out && file.contents) {
        const contents = file.contents.toString()
        if (contents.includes('<script>location=')) {
          file.contents = Buffer.from(contents
            .replace(
              /<script>location="([^"]+)"<\/script>/,
              '<script>location="$1" + window.location.search</script>'
            ))
        }
      }
    })
  })
}
