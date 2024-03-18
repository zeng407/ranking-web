@if ($message = Session::get('success'))
<span class="alert alert-success alert-dismissible alert-message">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
  
@if ($message = Session::get('error'))
<span class="alert alert-danger alert-dismissible alert-message">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
   
@if ($message = Session::get('warning'))
<span class="alert alert-warning alert-dismissible alert-message">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
   
@if ($message = Session::get('info'))
<span class="alert alert-info alert-dismissible alert-message">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    <strong class="m-2">{{ $message }}</strong>
</span>
@endif
  
@if ($errors->any())
<span class="alert alert-danger alert-dismissible alert-message">
    <button type="button" class="close" data-dismiss="alert">×</button>    
    {{__('Some problems occurred, please check the form below for errors')}}
</span>
@endif