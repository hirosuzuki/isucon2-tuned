SET NAMES 'utf8';

UPDATE stock SET order_id = NULL;
UPDATE variation SET sold_count = 0;
DELETE FROM order_request;