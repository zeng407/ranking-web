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

        if (!container) {
          return;
        }

        var wrapper = container.closest('.gam-togawa-ad');

        function initializeVisibleSlot() {
          if (initialized || !container.isConnected || container.getClientRects().length === 0) {
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
            if (event.slot === slot && event.isEmpty && wrapper) {
              wrapper.style.setProperty('display', 'none', 'important');
              wrapper.setAttribute('aria-hidden', 'true');
            }
          });

          if (!window.__rankingWebGptServicesEnabled) {
            googletag.enableServices();
            window.__rankingWebGptServicesEnabled = true;
          }

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
