# sudo aptitude install -y python-flask python-mysqldb python-routes
from __future__ import with_statement

try:
    import MySQLdb
    from MySQLdb.cursors import DictCursor
except ImportError:
    import pymysql as MySQLdb
    from pymysql.cursors import DictCursor

from flask import (
        Flask, request, redirect,
        render_template_string,
        render_template, _app_ctx_stack, Response
        )

# import googlecloudprofiler
import json, os
import redis
import gzip

config = {}

app = Flask(__name__, static_url_path='')

HTML_BASE = '/var/www/cached'

def load_config():
    global config
    print "Loading configuration"
    env = os.environ.get('ISUCON_ENV') or 'local'
    with open('../config/common.' + env + '.json') as fp:
        config = json.load(fp)

def connect_db():
    global config
    host = config['database']['host']
    port = config['database']['port']
    username = config['database']['username']
    password = config['database']['password']
    dbname   = config['database']['dbname']
    db = MySQLdb.connect(host=host, port=port, db=dbname, user=username, passwd=password, cursorclass=DictCursor, charset="utf8")
    return db

def init_db():
    print "Initializing database"
    with connect_db() as cur:
        with open('../config/database/initial_data.sql') as fp:
            for line in fp:
                line = line.strip()
                if line:
                    cur.execute(line)

def get_recent_sold():
    cur = get_db().cursor()
    cur.execute('''SELECT stock.seat_id, variation.name AS v_name, ticket.name AS t_name, artist.name AS a_name FROM stock
        JOIN variation ON stock.variation_id = variation.id
        JOIN ticket ON variation.ticket_id = ticket.id
        JOIN artist ON ticket.artist_id = artist.id
        WHERE order_id IS NOT NULL
        ORDER BY order_id DESC LIMIT 10''')
    recent_sold = cur.fetchall()
    cur.close()
    return recent_sold


def get_db():
    top = _app_ctx_stack.top
    if not hasattr(top, 'db'):
        top.db = connect_db()
    return top.db

cache = {}

def get_redis():
    if not 'redis' in cache:
        cache['redis'] = redis.Redis(host='localhost', port=6379, db=0)
    return cache['redis']

def get_variation():
    if not 'variation' in cache:
        variation = {}
        cur = get_db().cursor()
        cur.execute('''select variation.*, ticket.name as ticket_name, artist_id, artist.name as artist_name,
            (select min(id) from stock where stock.variation_id = variation.id) as min_stock_id
            from variation, ticket, artist
            where variation.ticket_id = ticket.id and ticket.artist_id = artist.id
            order by variation.id''')
        for row in cur.fetchall():
            variation[row['id']] = row
        print(variation)
        cur.close()
        cache['variation'] = variation
    return cache['variation']

@app.teardown_appcontext
def close_db_connection(exception):
    top = _app_ctx_stack.top
    if hasattr(top, 'db'):
        top.db.close()

def get_artists():
    if not 'artists' in cache:
        cur = get_db().cursor()
        cur.execute('SELECT * FROM artist')
        artists = cur.fetchall()
        print(artists)
        cache['artists'] = artists
        cur.close()
    return cache['artists']

def render_to_string(template_name, **context):
    result = render_template_string(file('templates/' + template_name).read().decode('utf-8'), **context)
    return result

def get_stocks(id):
    if not id in cache:
        cur = get_db().cursor()
        cur.execute(
            'SELECT seat_id, null as order_id FROM stock WHERE variation_id = %s order by id',
            (id,)
        )
        cache[id] = cur.fetchall()
        cur.close()
    return cache[id]

def get_ticket(ticket_id):
    if not id in cache:
        cur = get_db().cursor()
        cur.execute(
            'SELECT t.*, a.name AS artist_name FROM ticket t INNER JOIN artist a ON t.artist_id = a.id WHERE t.id = %s LIMIT 1',
            (ticket_id,)
        )
        cache[id] = cur.fetchone()
        cur.close()
    return cache[id]


@app.route("/buy", methods=['POST'])
def buy_page():

    variation_id = int(request.values['variation_id'])
    member_id = request.values['member_id']

    variation = get_variation()

    db = get_db()
    cur = db.cursor()
    cur.execute(
        'INSERT INTO order_request (member_id) VALUES (%s)',
        (member_id,)
    )
    order_id = db.insert_id()
    rows = cur.execute(
        'UPDATE stock SET order_id = %s WHERE variation_id = %s AND order_id IS NULL ORDER BY id LIMIT 1',
        (order_id, variation_id)
    )
    cur.execute(
        'UPDATE variation SET sold_count = sold_count + 1 WHERE id = %s',
        (variation_id,)
    )
    if rows > 0:
        cur.execute(
            'SELECT seat_id FROM stock WHERE order_id = %s LIMIT 1',
            (order_id,)
        );
        stock = cur.fetchone()
        db.commit()
    
        sidebar = get_side_html()
        create_top_html(sidebar)
        create_ticket_html(variation[variation_id]['ticket_id'], sidebar)
        create_artist_html(variation[variation_id]['artist_id'], sidebar)

        return render_template('complete.html', seat_id=stock['seat_id'], member_id=member_id)
    else:
        db.rollback()
        
        sidebar = get_side_html()
        create_top_html(sidebar)
        create_ticket_html(variation[variation_id]['ticket_id'], sidebar)
        create_artist_html(variation[variation_id]['artist_id'], sidebar)

        return render_template('soldout.html')

def file_write(filename, text):
    tmpFile = HTML_BASE + '/tmp.html.%d' % os.getpid()
    f = open(tmpFile, 'w')
    f.write(gzip.compress(text.encode('utf-8')))
    f.flush()
    os.fsync(f.fileno()) 
    f.close()
    os.rename(tmpFile, filename + ".gz")

def get_side_html():
    return render_to_string('side.html', recent_sold=get_recent_sold())

def create_top_html(sidebar):
    artists = get_artists()
    html = render_to_string('index.html', artists=artists, sidebar=sidebar)
    file_write(HTML_BASE + '/index.html', html)

def create_ticket_html(ticket_id, sidebar):
    cur = get_db().cursor()
   
    ticket = get_ticket(ticket_id)

    cur.execute(
        'SELECT id, name, sold_count FROM variation WHERE ticket_id = %s',
        (ticket_id,)
    )
    variations = cur.fetchall()

    for variation in variations:
        stocks = get_stocks(variation['id'])
        variation['vacancy'] = len(stocks) - variation['sold_count']
        variation['stock'] = {}
        i = 0
        for stock in stocks:
            variation['stock'][stock['seat_id']] = None if i >= variation['sold_count'] else "xxx"
            i += 1

    html = render_to_string('ticket.html', ticket=ticket, variations=variations, sidebar=sidebar)
    file_write(HTML_BASE + '/ticket/%d' % ticket_id, html)

def create_artist_html(artist_id, sidebar):
    cur = get_db().cursor()

    cur.execute('SELECT id, name FROM artist WHERE id = %s LIMIT 1', (artist_id,))
    artist = cur.fetchone()

    cur.execute('SELECT id, name FROM ticket WHERE artist_id = %s', (artist_id,))
    tickets = cur.fetchall()

    for ticket in tickets:
        cur.execute(
            '''SELECT sum(4096 - sold_count) as cnt FROM variation
                WHERE variation.ticket_id = %s''',
            (ticket['id'],)
        )
        ticket['count'] = cur.fetchone()['cnt']

    cur.close()

    html = render_to_string('artist.html', artist=artist, tickets=tickets, sidebar=sidebar)
    file_write(HTML_BASE + '/artist/%d' % artist_id, html)

def init_cache_html():
    variation = get_variation()
    sidebar = get_side_html()
    create_top_html(sidebar)
    ticket_ids = set([e['ticket_id']  for e in variation.values()])
    for ticket_id in ticket_ids:
        create_ticket_html(ticket_id, sidebar)
    artist_ids = set([e['artist_id']  for e in variation.values()])
    for artist_id in artist_ids:
        create_artist_html(artist_id, sidebar)

@app.route("/admin", methods=['GET', 'POST'])
def admin_page():
    if request.method == 'POST':
        init_db()
        init_cache_html()
        return redirect("/admin")
    else:
        init_cache_html()
        return render_template('admin.html')

@app.route("/admin/order.csv")
def admin_csv():
    cur = get_db().cursor()
    cur.execute('''SELECT order_request.*, stock.seat_id, stock.variation_id, stock.updated_at
         FROM order_request JOIN stock ON order_request.id = stock.order_id
         ORDER BY order_request.id ASC''')
    orders = cur.fetchall()
    cur.close()

    body = ''
    for order in orders:
        body += ','.join([str(order['id']), order['member_id'], order['seat_id'], str(order['variation_id']), order['updated_at'].strftime('%Y-%m-%d %X')])
        body += "\n"
    return Response(body, content_type="text/csv")

if __name__ == "__main__":
    load_config()
    port = int(os.environ.get("PORT", '5000'))
    app.run(debug=1, host='0.0.0.0', port=port)
else:
    """
    googlecloudprofiler.start(
        service='hello-profiler',
        service_version='1.0.1',
        verbose=3,
        # project_id='my-project-id'
    )
    """
    load_config()

