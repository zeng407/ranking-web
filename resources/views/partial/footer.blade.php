<footer class="bg-dark text-center text-white mt-auto">
  <!-- Copyright -->
  <div class="text-center p-3" style="background-color: rgba(0, 0, 0, 0.2);">
    © {{now()->year}} Copyright <a class="text-white" href="{{config('app.url')}}">{{config('app.url')}}</a> | Email <a class="text-white" href="mailto:{{config('app.contact_email')}}">{{config('app.contact_email')}}</a>
    @if(config('social.show_facebook'))| <a href="{{config('social.facebook_page_url')}}" target="_blank" class="text-white"><i class="fab fa-facebook-f"></i> Facebook</a> @endif
    | <a href="{{config('app.feedback_form_url')}}" target="_blank" class="text-white">{{__('Feedback')}}</a>
  </div>
</footer>
