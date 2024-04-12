@extends('layouts.app', [
    'title' => __('Terms of Service'),
    'ogImage' => asset('/storage/og-image.jpeg'),
    'stickyNav' => 'sticky-top',
])

@section('content')
    <div class="container">
        <div class="text-center mt-2">
            <h1>Terms of Service</h1>
        </div>
        <p>Welcome to {{ config('app.name') }} (hereinafter referred to as "the Site") and its services. By completing the registration or starting to use the services, you acknowledge that you have read, understood, and agreed to be bound by all the provisions of these Terms of Service. If you do not agree with the content of these Terms of Service or if your country or region excludes any or all of the provisions of these Terms of Service, you must immediately cease using the services. Additionally, when using specific features of the services, you may be subject to additional terms and conditions announced by the Site for those specific features. Such additional terms and conditions are also considered part of these Terms of Service. The Site reserves the right to modify or change the content of these Terms of Service at any time and will announce such modifications or changes on the Site. By continuing to use the services after any modifications or changes, you are deemed to have read, understood, and agreed to accept such modifications or changes.
        </p>

        <h3>Privacy Policy</h3>
        <ul>The Site places great importance on the protection of privacy. Your membership registration and other specific information will be collected and used in accordance with the "Privacy Policy" of the services. By using the services, you acknowledge and agree that the Site may collect and use your personal information in accordance with the provisions of the "Privacy Policy," such as for online activities and surveys.
        </ul>

        <h3>General Provisions</h3>
        <ul>These Terms of Service constitute the complete agreement between you and the Site regarding your use of the services, superseding any prior agreements between you and the Site regarding the services. The interpretation and application of these Terms of Service, as well as any disputes related to these Terms of Service, shall be governed by the laws of the Republic of China (Taiwan) and shall be subject to the jurisdiction of the Taipei District Court, unless otherwise provided by law. The failure of the Site to exercise or enforce any right or provision of these Terms of Service shall not constitute a waiver of such right or provision. If any provision of these Terms of Service is found by a court of competent jurisdiction to be invalid, the parties agree that the court should endeavor to give effect to the parties' intentions as expressed in the provision, and the other provisions of these Terms of Service shall remain in full force and effect.
        </ul>

        <h3>Terms of Use and Obligations</h3>
        <ul>In addition to complying with the provisions of these Terms of Service, you agree to comply with relevant laws and regulations of the Republic of China (Taiwan), other service terms of this service, and agree not to engage in the following activities: <br>
            If you are a user outside the Republic of China (Taiwan), you agree to comply with the relevant laws of your country or region. You agree and warrant not to use this service for illegal or harmful activities that infringe upon the rights of others.<br>
            <br>
            Impersonating others when using this service.<br>
            Infringing upon the reputation, privacy rights, copyrights, intellectual property rights, and other rights of others.<br>
            Uploading, posting, or publishing any false, defamatory, insulting, threatening, offensive, immoral, criminal, or other unlawful text, images, or any form of files.<br>
            Violating confidentiality obligations under applicable laws or contracts.<br>
            Uploading, posting, or publishing any code or data containing computer viruses, malware, or any code or data that may cause interruption, destruction, or limitation of the functionality of computer software or hardware.<br>
            Engaging in the sale of firearms, drugs, controlled substances, pirated software, other prohibited items, or any other illegal trading activities.<br>
            Disrupting and interfering with the data, activities, or functions provided by this service, or attempting to access, intrude, or damage any system of this service, or engaging in any behavior that violates or damages this service.<br>
            Collecting others' email addresses and other personal information without consent.<br>
            Any other behavior that this site has justifiable reasons to deem inappropriate.<br>
            If you violate or are suspected of violating the above matters, this site has the right to terminate or restrict your use of the account (or any part thereof) or this service, with or without notice. You agree that this site shall not be liable to you or any third party for any termination or restriction of your use of this service.<br>
        </ul>
        
        <h3>Contributions to the Service</h3>
        <ul>
            If you provide any ideas, suggestions, or proposals (hereinafter referred to as "Contributions") to the Service, you understand and agree that:<br>
            <br>
            The Contributions are not considered confidential or proprietary information and do not infringe upon the reputation, privacy rights, copyrights, intellectual property rights, or other rights of others.<br>
            The Service has no express or implied obligation of confidentiality with respect to the Contributions.<br>
            The Service may already have similar ideas or proposals under consideration or in development.<br>
            The Service may use (or choose not to use) the Contributions in any manner, on any media, for the purpose of public interest or promoting the Service.<br>
            The Contributions automatically become the property of the Service without any liability to you. You have no right to claim any form of compensation or remuneration from the Service under any circumstances.<br>
        </ul>

        <h3>Prohibited Commercial Use</h3>
        <ul>You agree and undertake not to use or access the systems provided by this service (including but not limited to member content and member accounts) for any commercial purposes.</ul>

        <h3>Authorization to the Service</h3>
        <ul>If you do not have the legal right to authorize others to use, modify, distribute, reproduce, or publicly display certain data, please do not upload, input, or provide such data to this service. Any data uploaded, inputted, or provided by you to this service shall remain your property or the property of your authorized party. However, by uploading, inputting, or providing any data to this service, you agree to grant this site the right to use, modify, distribute, reproduce, or publicly display such data for public interest or for the purpose of promoting and advertising this service. You also grant this site the right to sublicense such rights to third parties within this scope. You further warrant that the use, modification, distribution, reproduction, public display, or sublicensing of such data by this service will not infringe upon any rights of third parties. Otherwise, you shall be liable for any damages incurred by this site.
        </ul>

        <h3>System Interruption or Failure</h3>
        <ul>This service may experience interruptions or failures, which may cause inconvenience, data loss, errors, or other issues in your usage. It is advisable for you to take protective measures when using this service. This site shall not be liable for any damages incurred by you due to the use (or inability to use) this service.</ul>

        <h3>Links to Third-Party Websites and Third-Party Content</h3>
        <ul>This service may provide links to websites of other organizations or entities (hereinafter referred to as "third parties"). The websites of third parties are the responsibility of the respective organizations or entities and are beyond the control and responsibility of this service. This site does not guarantee the validity, accuracy, suitability, or completeness of any search results or external links. You may come across inappropriate or unnecessary websites during your search or visit. In such cases, this service recommends that you refrain from browsing or promptly leave such websites. You also agree that this site shall not be liable for any damages arising from your access to non-service websites linked by this service, and shall not be responsible for any compensation for damages.
            This service may collaborate with third parties (hereinafter referred to as "content providers"), such as government agencies, to provide various content, including laws, regulations, news, information, and events, for publication on this service. The source of the content will be indicated when published by this service. Out of respect for the intellectual property rights of the content providers, this service does not conduct substantive review or modification of the provided content. You should independently judge the accuracy and authenticity of such content, and this site shall not be responsible for the accuracy or authenticity of such content. If you believe that certain content is inappropriate, infringing, or inaccurate, please directly contact the respective content provider to express your concerns.
        </ul>

        <h3>Service Changes and Notifications</h3>
        <ul>You agree that this site reserves the right to modify, temporarily or permanently suspend, or discontinue the provision of this service (or any part thereof) at any time without prior notice. When required by law or other relevant regulations, this service may provide notifications, including but not limited to posting on this service's webpages, sending emails, or using other reasonable methods, to inform you of changes to these Terms of Service. However, if you violate these Terms of Service by unauthorized access to this service or by providing incorrect or outdated information during registration, you will not receive such notifications. By accessing this service through authorized means, you agree that any and all notifications provided to you by this service shall be deemed as delivered.
        </ul>

        <h3>Protection of Intellectual Property Rights</h3>
        <ul>The programs, software, and website content used in this service, including but not limited to information, data, images, files, website architecture, web design, and member content, are owned by this site or other rights holders in accordance with the law. This includes, but is not limited to, patent rights, copyright, and proprietary technology. No one may modify, reproduce, distribute, publish, publicly display, reverse engineer, or disassemble them without the express written consent of this site or other rights holders. If you wish to quote or reproduce the aforementioned materials, except as expressly permitted by law, you must obtain the prior written consent of this site or other rights holders. Any violation of this provision may result in liability for damages.
        </ul>

        <h3>Handling of Intellectual Property Rights or Copyright Infringement</h3>
        <ul>This site respects the intellectual property rights of others and also requires users of this service to respect the intellectual property rights of others. This service may suspend or terminate the accounts of users who may be infringing. If you believe that your copyright or intellectual property rights have been infringed, please provide the following information to this service:<br>
            <br>
            Your accurate information and contact details, and consent to provide such information to the reported party in case of objections.
            Proof of being the legal representative of the copyright or intellectual property rights holder.
            Description of the infringed work or other intellectual property rights, as well as a description of the infringed data.
            A statement that you have a good faith belief that the use is not authorized by the copyright owner, its agent, or the law.
            A statement, under penalty of perjury, that the information in your notice is accurate and that you are authorized to act on behalf of the copyright owner.
        </ul>
    @endsection
    @section('footer')
        @include('partial.footer')
    @endsection
