# Ad slots (AdSense) — deployment notes

Ads are configured at runtime, not at build time. The frontend reads
`window.__APP_CONFIG__.ads` from `public/app-config.js`, which nginx serves with
`Cache-Control: no-store`, so slot ids can change without rebuilding the bundle.

Only Google AdSense is wired up. There is no sponsor "ad-free" path and no OneAd.

## Turning ads on

Edit the deployed `app-config.js` (image path `/usr/share/nginx/html/app-config.js`):

```js
ads: {
  publisherId: 'ca-pub-XXXXXXXXXXXXXXXX',
  slots: {
    homeTop: '',        // banner under the tag strip on the home feed
    homeRail: '',       // 300x250 in the right-hand discovery rail
    homeRailBottom: '', // 300x600 at the bottom of the rail, wide layout only
    homeFeed: '',       // native card inside the vote grid, every 6th card
    homeFooter: '',     // banner below the "load more" button
    rankList: '',       // in-content, after the 10th community ranking row
    gameResult: '',     // below the winner hero on a finished game
  },
},
```

Rules the code enforces:

- An empty `publisherId` renders no ad markup anywhere.
- An empty slot id turns off that one position only; the others keep working.
- Each slot reserves its height before the unit loads, so a late fill does not
  push the content the reader is looking at.
- A unit is only requested once it is within 200px of the viewport, so slots far
  down the page do not burn unfilled impressions.
- `homeRailBottom` (the tall unit) is hidden below 921px, where the layout is a
  single column.

Slot ids are public identifiers, not secrets — they belong in `app-config.js`,
never in the build or in the repository's committed config.

## 18+ content rule — do not bypass

AdSense allows its units neither **on** adult content nor **beside** it, and the
penalty is the whole account, not the page. The frontend therefore:

- renders **no ad at all** on a censored post's game page and ranking page
  (`GameDefinition.is_censored`), and
- slides the home in-feed slot further down the grid whenever the card above or
  below it is censored, dropping the slot entirely when the loaded feed has no
  clean neighbours left (`PublicPost.is_censored`).

If a new page or list starts showing posts, it must apply the same two rules
before it gets a slot.

## Checks after a deployment

1. `curl -sI https://<host>/app-config.js` returns `Cache-Control: no-store`.
2. The home page shows five labelled slots at desktop width, four at phone width
   (the tall rail unit is hidden).
3. A ranking page with more than ten rows breaks the list once, after row ten.
4. An 18+ post's game and ranking pages contain no `.ad-slot` element at all.
