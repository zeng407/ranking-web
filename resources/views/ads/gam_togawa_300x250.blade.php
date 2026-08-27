@php
  $togawaAdUnit = config('services.google_ad.togawa_html.ad_unit');
  $togawaSlotId = 'div-gpt-ad-togawa-300x250';
@endphp

@if (config('services.google_ad.enabled') &&
        config('services.google_ad.togawa_html.enabled') &&
        $togawaAdUnit &&
        !is_skip_ad())
  <div class="gam-togawa-ad justify-content-center my-3 w-100"
    style="display: flex;" aria-label="Advertisement">
    <div id="{{ $togawaSlotId }}"
      style="width: 300px; min-width: 300px; min-height: 250px; display: block;">
    </div>
  </div>

  @push('scripts')
    <script data-cfasync="false">
      window.googletag = window.googletag || { cmd: [] };
      googletag.cmd.push(function() {
        var slotId = @json($togawaSlotId);
        var container = document.getElementById(slotId);
        var initialized = false;
        var visibilityObserver = null;
        var creativeLoadTimer = null;
        var retryScheduled = false;
        var retryCount = 0;
        var creativeLoaded = false;
        var lastRequestAt = 0;
        var MAX_RETRY_COUNT = 1;
        var RETRY_DELAY_MS = 30 * 1000;
        var CREATIVE_LOAD_TIMEOUT_MS = 10 * 1000;

        if (!container) {
          return;
        }

        function isContainerVisible() {
          return container.isConnected &&
            container.getClientRects().length > 0 &&
            document.visibilityState === 'visible';
        }

        function scheduleRetry(slot) {
          if (retryScheduled || retryCount >= MAX_RETRY_COUNT) {
            return;
          }

          retryScheduled = true;
          var elapsed = Date.now() - lastRequestAt;
          var delay = Math.max(0, RETRY_DELAY_MS - elapsed);

          window.setTimeout(function() {
            retryScheduled = false;

            if (!isContainerVisible()) {
              return;
            }

            retryCount += 1;
            creativeLoaded = false;
            lastRequestAt = Date.now();
            googletag.pubads().refresh([slot]);
          }, delay);
        }

        function initializeVisibleSlot() {
          if (initialized || !isContainerVisible()) {
            return false;
          }

          var slot = googletag.defineSlot(@json($togawaAdUnit), [300, 250], slotId);

          if (!slot) {
            return false;
          }

          initialized = true;
          if (visibilityObserver) {
            visibilityObserver.disconnect();
          }

          slot.addService(googletag.pubads());
          googletag.pubads().addEventListener('slotRenderEnded', function(event) {
            if (event.slot !== slot) {
              return;
            }

            window.clearTimeout(creativeLoadTimer);

            if (event.isEmpty) {
              scheduleRetry(slot);
              return;
            }

            creativeLoaded = false;
            creativeLoadTimer = window.setTimeout(function() {
              if (!creativeLoaded) {
                scheduleRetry(slot);
              }
            }, CREATIVE_LOAD_TIMEOUT_MS);
          });
          googletag.pubads().addEventListener('slotOnload', function(event) {
            if (event.slot !== slot) {
              return;
            }

            creativeLoaded = true;
            window.clearTimeout(creativeLoadTimer);
          });

          if (!window.__rankingWebGptServicesEnabled) {
            googletag.enableServices();
            window.__rankingWebGptServicesEnabled = true;
          }

          lastRequestAt = Date.now();
          googletag.display(slotId);
          return true;
        }

        if (!initializeVisibleSlot()) {
          visibilityObserver = new MutationObserver(initializeVisibleSlot);
          visibilityObserver.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ['class', 'style', 'v-cloak'],
            subtree: true
          });
          window.addEventListener('load', initializeVisibleSlot, { once: true });
        }
      });
    </script>
  @endpush
@endif
