// This file is intentionally not hashed. Deploy it with Cache-Control: no-store
// so an environment can change API routing without rebuilding the frontend.
window.__APP_CONFIG__ = Object.freeze({
  apiBaseUrl: '/api/v1',
  contactEmail: '',
  // AdSense. An empty publisherId turns every slot off, and a slot left empty turns
  // that one position off, so a deployment can move ads without a rebuild.
  //
  // Ads never render on a censored (18+) post's game or ranking page, and the home
  // feed slot slides past 18+ cards: AdSense allows neither, and the penalty is the
  // whole account.
  ads: {
    publisherId: '',
    slots: {
      homeTop: '',
      homeRail: '',
      homeRailBottom: '',
      homeFeed: '',
      homeFooter: '',
      rankList: '',
      gameResult: '',
    },
  },
  // The realtime endpoint for the live game room leaderboard. Leave `key` empty to
  // disable the websocket; the room then falls back to its poll and stays playable.
  // host and secure default to the page's own, which is right when the websocket is
  // proxied alongside the API.
  realtime: {
    key: '',
    host: '',
    port: 6001,
    cluster: '',
  },
})
