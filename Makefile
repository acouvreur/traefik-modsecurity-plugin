NEXT = unreleased
update-doc-version:
	sed -i 's/version=v.*$$/version=v$(NEXT)/g' docker-compose.yml