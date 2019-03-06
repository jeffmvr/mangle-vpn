
.PHONY: build
build: build-ui
	$(CURDIR)/manage.py makemigrations


.PHONY: build-ui
build-ui:
	rm -rf $(CURDIR)/ui/public
	cd $(CURDIR)/ui && npm install && npm run build


.PHONY: install
install:
	sudo $(CURDIR)/install.sh


.PHONY: update
update:
	git fetch origin master && git reset --hard origin/master
	pip install -r $(CURDIR)/requirements.txt > /dev/null
	$(CURDIR)/manage.py migrate
	systemctl restart mangle-web
