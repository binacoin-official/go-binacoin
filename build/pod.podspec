Pod::Spec.new do |spec|
  spec.name         = 'Gbina'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/binacoin-official/go-binacoin'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS Binacoin Client'
  spec.source       = { :git => 'https://github.com/binacoin-official/go-binacoin.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/Gbina.framework'

	spec.prepare_command = <<-CMD
    curl https://gbinastore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/Gbina.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
