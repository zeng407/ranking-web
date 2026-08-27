@php
  $togawaAdUnit = config('services.google_ad.togawa_html.ad_unit');
  $togawaSlotId = 'div-gpt-ad-togawa-300x250';
  $togawaDisplayEvent = $displayEvent ?? null;
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
      (function() {
        var slotId = @json($togawaSlotId);
        var displayEvent = @json($togawaDisplayEvent);
        var displayRequested = !displayEvent;
        var displaySlot = null;

        function requestDisplay() {
          displayRequested = true;
          if (displaySlot) {
            displaySlot();
          }
        }

        if (displayEvent) {
          window.addEventListener(displayEvent, requestDisplay, { once: true });
        }

        googletag.cmd.push(function() {
          var container = document.getElementById(slotId);

          if (!container) {
            return;
          }

          var wrapper = container.closest('.gam-togawa-ad');
          var slot = googletag.defineSlot(@json($togawaAdUnit), [300, 250], slotId);

          if (!slot) {
            return;
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

          displaySlot = function() {
            if (!displaySlot.displayed) {
              displaySlot.displayed = true;
              googletag.display(slotId);
            }
          };

          if (displayRequested) {
            displaySlot();
          }
        });
      })();
    </script>
  @endpush
@endif
